#!/usr/bin/env python3
"""Reproduce and refresh the personal patch stack without rewriting a branch."""

import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parent.parent
MANIFEST = ROOT / "fork" / "series.json"
README_MARKER = "\n## Personal fork features\n"


def git(*args, cwd=None, capture=True, input=None):
    return subprocess.run(
        ["git", "-c", "core.hooksPath=/dev/null", *args], cwd=cwd or ROOT,
        input=input, stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None, check=True,
    ).stdout


def resolve(ref):
    return git("rev-parse", "--verify", f"{ref}^{{commit}}").decode().strip()


def read_series():
    data = json.loads(MANIFEST.read_text())
    if data["version"] != 1 or not data["patches"]:
        raise ValueError("Unsupported or empty patch series")
    ids = set()
    for patch in data["patches"]:
        if patch["id"] in ids:
            raise ValueError("Duplicate patch id")
        ids.add(patch["id"])
        name = patch["file"]
        if not re.fullmatch(r"[0-9]{4}-[a-z0-9-]+\.patch", name):
            raise ValueError(f"Invalid patch filename: {name}")
        raw = (MANIFEST.parent / "patches" / name).read_bytes()
        if hashlib.sha256(raw).hexdigest() != patch["sha256"]:
            raise ValueError(f"Patch checksum differs: {name}")
    for patch in data["patches"]:
        if not set(patch.get("depends_on", [])) <= ids:
            raise ValueError(f"Unknown dependency for {patch['id']}")
    return data


def check_migrations(data, target):
    """A clean text application is not evidence of safe SQLite numbering."""
    names = git("ls-tree", "-r", "--name-only", target,
                "backend/internal/storage/sqlite/migrations").decode().splitlines()
    by_number = {Path(name).name.split("_", 1)[0]: name for name in names}
    for patch in data["patches"]:
        for name in patch["added_migrations"]:
            number = Path(name).name.split("_", 1)[0]
            if number in by_number:
                raise ValueError(
                    f"Migration collision: {name} / {by_number[number]}. "
                    "Port the ownership migration, its legacy repair mapping, "
                    "and generated SQL together before retrying."
                )


def export_series(template, base, tip, directory):
    """Write a NEW bundle from the ordered commits, keeping metadata by subject."""
    base, tip = resolve(base), resolve(tip)
    git("merge-base", "--is-ancestor", base, tip)
    commits = git("rev-list", "--reverse", f"{base}..{tip}").decode().splitlines()
    if len(commits) != len(template["patches"]):
        raise ValueError("Expected exactly one commit per feature; squash fixups first")
    if directory.exists():
        raise ValueError(f"Output directory already exists: {directory}")
    records = []
    contents = []
    for index, (commit, old) in enumerate(zip(commits, template["patches"]), 1):
        subject = git("show", "-s", "--format=%s", commit).decode().strip()
        if subject != old["subject"]:
            raise ValueError(f"Unexpected commit/order for {old['id']}: {subject}")
        if len(git("show", "-s", "--format=%P", commit).split()) != 1:
            raise ValueError("Patch stack must be linear, without merge commits")
        raw = git("format-patch", "-1", "--stdout", "--no-signature", "--full-index", "--binary", commit)
        name = f"{index:04d}-{old['id']}.patch"
        added = git("diff-tree", "--no-commit-id", "--name-only", "-r", "--diff-filter=A",
                    commit, "--", "backend/internal/storage/sqlite/migrations").decode().splitlines()
        records.append({**old, "commit": commit, "file": name,
                        "sha256": hashlib.sha256(raw).hexdigest(), "added_migrations": added})
        contents.append((name, raw))
    directory.mkdir(parents=True)
    (directory / "patches").mkdir()
    for name, raw in contents:
        (directory / "patches" / name).write_bytes(raw)
    result = {**template, "base": base, "source_head": tip, "patches": records}
    (directory / "series.json").write_text(json.dumps(result, indent=2) + "\n")
    return result


def apply_series(data, base, worktree, branch=None):
    base = resolve(base)
    check_migrations(data, base)
    if worktree.exists():
        raise ValueError(f"Worktree destination already exists: {worktree}")
    if branch:
        git("check-ref-format", "--branch", branch)
        git("worktree", "add", "-b", branch, str(worktree), base, capture=False)
    else:
        git("worktree", "add", "--detach", str(worktree), base, capture=False)
    for patch in data["patches"]:
        print(f"Applying {patch['id']}", flush=True)
        try:
            git("am", "--3way", str(MANIFEST.parent / "patches" / patch["file"]),
                cwd=worktree, capture=False)
        except subprocess.CalledProcessError:
            print(f"Conflict kept in {worktree}. Resolve and run git am --continue there. "
                  "Remaining patch filenames/order are in fork/series.json; "
                  "the original checkout is unchanged.", file=sys.stderr)
            raise


def check_source(data):
    if git("status", "--porcelain").strip():
        raise ValueError("Commit and export all source changes, including new files, first")
    # Maintenance files are outside the functional stack by design.
    git("diff", "--exit-code", data["source_head"], "HEAD", "--", ".",
        ":(exclude)fork/**", ":(exclude)README.md", ":(exclude)docs/personal-fork.md",
        ":(exclude)docs/superpowers/plans/2026-10-04-personal-patch-stack.md")
    original_readme = git("show", f"{data['source_head']}:README.md").decode().rstrip()
    current_readme = (ROOT / "README.md").read_text().split(README_MARKER, 1)[0].rstrip()
    if current_readme != original_readme:
        raise ValueError("Upstream README content changed outside the personal appendix")


def verify(data):
    check_source(data)
    temp = Path(tempfile.mkdtemp(prefix="ao-stack-verify-"))
    worktree = temp / "checkout"
    try:
        apply_series(data, data["base"], worktree)
        expected = git("rev-parse", f"{data['source_head']}^{{tree}}").strip()
        actual = git("rev-parse", "HEAD^{tree}", cwd=worktree).strip()
        if expected != actual:
            raise ValueError("Replayed source tree differs from the feature stack")
        git("worktree", "remove", str(worktree))
        temp.rmdir()
        print("Verified: checksums, runtime source, and exact replayed Git tree")
    except Exception:
        print(f"Verification checkout retained for inspection: {worktree}", file=sys.stderr)
        raise


def replay(data, args):
    check_source(data)
    target = resolve(args.onto)
    git("merge-base", "--is-ancestor", data["base"], target)
    source_readme = (ROOT / "README.md").read_text()
    if README_MARKER not in source_readme:
        raise ValueError("Missing personal README appendix")
    if "fork" in git("ls-tree", "--name-only", target, "fork").decode().splitlines():
        raise ValueError("Target upstream owns fork/: reconcile the maintenance directory manually")
    worktree = args.worktree.resolve()
    apply_series(data, target, worktree, args.branch)
    carry_maintenance(data, target, worktree)
    print(f"Prepared {worktree} on {args.branch}. Review the migration/version metadata, "
          "run the documented checks, then commit the maintenance files. Nothing was pushed.")


def carry_maintenance(data, target, worktree):
    source_readme = (ROOT / "README.md").read_text()
    if README_MARKER not in source_readme:
        raise ValueError("Missing personal README appendix")
    tip = git("rev-parse", "HEAD", cwd=worktree).decode().strip()
    # Carry the maintenance layer AFTER runtime features; never overwrite an
    # upstream README with an old full-file copy.
    destination = worktree / "fork"
    if destination.exists():
        raise ValueError("Target upstream owns fork/: keep the new checkout and reconcile manually")
    export_series(data, target, tip, destination)
    for source in MANIFEST.parent.iterdir():
        if source.name not in ("patches", "series.json", "__pycache__"):
            target_path = destination / source.name
            if source.is_dir():
                shutil.copytree(source, target_path)
            else:
                shutil.copy2(source, target_path)
    readme = worktree / "README.md"
    upstream_text = readme.read_text().split(README_MARKER, 1)[0].rstrip()
    readme.write_text(upstream_text + README_MARKER + source_readme.split(README_MARKER, 1)[1])
    personal_doc = ROOT / "docs" / "personal-fork.md"
    if personal_doc.exists():
        shutil.copy2(personal_doc, worktree / "docs" / "personal-fork.md")
    plan = ROOT / "docs/superpowers/plans/2026-10-04-personal-patch-stack.md"
    if plan.exists():
        destination_plan = worktree / plan.relative_to(ROOT)
        destination_plan.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(plan, destination_plan)


def finalize(data, args):
    check_source(data)
    worktree = args.worktree.resolve()
    common = Path(git("rev-parse", "--git-common-dir", cwd=worktree).decode().strip())
    own_common = Path(git("rev-parse", "--git-common-dir").decode().strip())
    if (worktree / common).resolve() != (ROOT / own_common).resolve():
        raise ValueError("Finalize requires a worktree of this repository")
    if git("status", "--porcelain", cwd=worktree).strip():
        raise ValueError("Finish all runtime patches in a clean worktree first")
    target = resolve(args.base)
    git("merge-base", "--is-ancestor", data["base"], target)
    carry_maintenance(data, target, worktree)
    print(f"Maintenance bundle refreshed in {worktree}; review, test, and commit it")


def contribution(data, args):
    check_source(data)
    by_id = {patch["id"]: patch for patch in data["patches"]}
    selected = set()

    def include(feature):
        if feature not in by_id:
            raise ValueError(f"Unknown feature: {feature}")
        if feature in selected:
            return
        if by_id[feature]["category"] != "upstream-candidate":
            raise ValueError(f"Personal-only patch cannot enter a contribution: {feature}")
        selected.add(feature)
        for dependency in by_id[feature].get("depends_on", []):
            include(dependency)

    include(args.feature)
    subset = {**data, "patches": [p for p in data["patches"] if p["id"] in selected]}
    target = resolve(args.onto or data["base"])
    git("merge-base", "--is-ancestor", data["base"], target)
    apply_series(subset, target, args.worktree.resolve(), args.branch)
    print("Prepared contribution candidate with dependencies; no local build, legacy "
          "migration repair, or fork maintenance layer. Run checks and review before a PR.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("status")
    commands.add_parser("verify")
    command = commands.add_parser("replay")
    command.add_argument("--onto", required=True)
    command.add_argument("--branch", required=True)
    command.add_argument("--worktree", type=Path, required=True)
    command = commands.add_parser("finalize")
    command.add_argument("--base", required=True)
    command.add_argument("--worktree", type=Path, required=True)
    command = commands.add_parser("contribution")
    command.add_argument("--feature", required=True)
    command.add_argument("--onto")
    command.add_argument("--branch", required=True)
    command.add_argument("--worktree", type=Path, required=True)
    command = commands.add_parser("export")
    command.add_argument("--base", required=True)
    command.add_argument("--tip", required=True)
    command.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        data = read_series()
        if args.command == "status":
            print(f"Upstream: {data['upstream']}\nPinned base: {data['base']}")
            for patch in data["patches"]:
                print(f"{patch['id']}: {patch['category']} — {patch['subject']}")
        elif args.command == "verify":
            verify(data)
        elif args.command == "replay":
            replay(data, args)
        elif args.command == "finalize":
            finalize(data, args)
        elif args.command == "contribution":
            contribution(data, args)
        else:
            export_series(data, args.base, args.tip, args.output.resolve())
            print(f"Exported new bundle to {args.output}")
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print(f"Stack operation failed: {error}", file=sys.stderr)
        if isinstance(error, subprocess.CalledProcessError) and error.stderr:
            print(error.stderr.decode(errors="replace"), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
