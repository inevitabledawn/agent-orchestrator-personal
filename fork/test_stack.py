"""Exercise real Git boundaries; no network, AO data, or provider processes."""

import argparse
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest


spec = importlib.util.spec_from_file_location("stack", Path(__file__).with_name("stack.py"))
stack = importlib.util.module_from_spec(spec)
spec.loader.exec_module(stack)


class StackTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="ao-stack-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "source"
        self.root.mkdir()
        self.old_root, self.old_manifest = stack.ROOT, stack.MANIFEST
        stack.ROOT = self.root
        stack.MANIFEST = self.root / "fork" / "series.json"
        self.addCleanup(self.restore_globals)
        self.git("init", "--quiet", "-b", "main")
        self.git("config", "user.name", "Stack Test")
        self.git("config", "user.email", "stack@example.invalid")
        self.write("app.txt", "upstream\n")
        self.write("README.md", "# Upstream\n")
        self.base = self.commit("chore: baseline")
        self.git("checkout", "--quiet", "-b", "personal/source")
        self.write("app.txt", "personal behavior\n")
        first = self.commit("feat: personal behavior")
        self.migration = "backend/internal/storage/sqlite/migrations/0171_ownership.sql"
        self.write(self.migration, "-- personal schema\n")
        self.tip = self.commit("feat: ownership")
        self.template = {"version": 1, "upstream": "example/upstream", "patches": [
            {"id": "behavior", "subject": "feat: personal behavior", "category": "upstream-candidate"},
            {"id": "ownership", "subject": "feat: ownership", "category": "upstream-candidate"},
        ]}
        self.data = stack.export_series(self.template, self.base, self.tip, self.root / "fork")
        self.write("fork/stack.py", Path(__file__).with_name("stack.py").read_text())
        self.write("README.md", "# Upstream\n" + stack.README_MARKER + "- Preserved feature.\n")
        self.commit("docs: maintenance")

    def restore_globals(self):
        stack.ROOT, stack.MANIFEST = self.old_root, self.old_manifest

    def git(self, *args):
        return subprocess.check_output(["git", "-c", "core.hooksPath=/dev/null", *args],
                                       cwd=self.root, stderr=subprocess.DEVNULL).decode().strip()

    def write(self, path, text):
        target = self.root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text)

    def commit(self, subject):
        self.git("add", "--all")
        self.git("commit", "--quiet", "-m", subject)
        return self.git("rev-parse", "HEAD")

    def args(self, onto=None):
        return argparse.Namespace(onto=onto or self.base, branch="personal/update",
                                  worktree=Path(self.temp.name) / "updated")

    def test_exact_replay_preserves_original_checkout(self):
        head = self.git("rev-parse", "HEAD")
        stack.verify(stack.read_series())
        self.assertEqual(head, self.git("rev-parse", "HEAD"))
        self.assertEqual("", self.git("status", "--porcelain"))

    def test_replay_carries_refreshed_bundle_and_readme_appendix(self):
        args = self.args()
        stack.replay(stack.read_series(), args)
        bundle = json.loads((args.worktree / "fork/series.json").read_text())
        self.assertEqual(self.base, bundle["base"])
        self.assertEqual("personal behavior\n", (args.worktree / "app.txt").read_text())
        self.assertEqual(1, (args.worktree / "README.md").read_text().count(stack.README_MARKER))
        self.assertTrue((args.worktree / "fork/stack.py").is_file())

    def test_tampered_patch_is_rejected_before_mutation(self):
        patch = self.root / "fork/patches" / self.data["patches"][0]["file"]
        patch.write_bytes(patch.read_bytes() + b"tamper")
        with self.assertRaisesRegex(ValueError, "checksum"):
            stack.read_series()
        self.assertFalse(self.args().worktree.exists())

    def test_migration_number_collision_stops_before_creating_worktree(self):
        self.git("checkout", "--quiet", "-b", "upstream-next", self.base)
        self.write("backend/internal/storage/sqlite/migrations/0171_other.sql", "-- upstream\n")
        target = self.commit("feat: other migration")
        self.git("checkout", "--quiet", "personal/source")
        args = self.args(target)
        with self.assertRaisesRegex(ValueError, "Migration collision"):
            stack.replay(self.data, args)
        self.assertFalse(args.worktree.exists())

    def test_existing_worktree_path_is_never_overwritten(self):
        args = self.args()
        args.worktree.mkdir()
        marker = args.worktree / "keep.txt"
        marker.write_text("keep")
        with self.assertRaisesRegex(ValueError, "already exists"):
            stack.replay(self.data, args)
        self.assertEqual("keep", marker.read_text())

    def test_runtime_change_requires_reexport(self):
        self.write("app.txt", "unexported behavior\n")
        self.commit("fix: change without updated patch")
        with self.assertRaises(subprocess.CalledProcessError):
            stack.verify(self.data)

    def test_conflict_is_kept_in_new_worktree(self):
        self.git("checkout", "--quiet", "-b", "upstream-next", self.base)
        self.write("app.txt", "conflicting upstream behavior\n")
        target = self.commit("feat: conflicting upstream")
        self.git("checkout", "--quiet", "personal/source")
        original = self.git("rev-parse", "HEAD")
        args = self.args(target)
        with self.assertRaises(subprocess.CalledProcessError):
            stack.replay(self.data, args)
        self.assertTrue(args.worktree.exists())
        self.assertEqual(original, self.git("rev-parse", "HEAD"))
        state = subprocess.check_output(["git", "rev-parse", "--git-path", "rebase-apply"],
                                        cwd=args.worktree).decode().strip()
        self.assertTrue(Path(state).is_dir())

    def test_export_rejects_maintenance_commit_in_feature_range(self):
        destination = Path(self.temp.name) / "invalid"
        with self.assertRaisesRegex(ValueError, "one commit per feature"):
            stack.export_series(self.data, self.base, "HEAD", destination)
        self.assertFalse(destination.exists())

    def test_contribution_omits_personal_only_patches(self):
        self.data["patches"][1]["category"] = "personal-only"
        args = self.args()
        args.feature, args.onto = "behavior", None
        stack.contribution(self.data, args)
        self.assertTrue((args.worktree / "app.txt").is_file())
        self.assertFalse((args.worktree / self.migration).exists())
        self.assertFalse((args.worktree / "fork").exists())

    def test_contribution_includes_declared_dependencies(self):
        self.data["patches"][1]["depends_on"] = ["behavior"]
        args = self.args()
        args.feature, args.onto = "ownership", None
        stack.contribution(self.data, args)
        self.assertEqual("personal behavior\n", (args.worktree / "app.txt").read_text())
        self.assertTrue((args.worktree / self.migration).exists())

    def test_finalize_reconstructs_maintenance_after_manual_application(self):
        args = self.args()
        stack.apply_series(self.data, self.base, args.worktree, args.branch)
        stack.finalize(self.data, argparse.Namespace(base=self.base, worktree=args.worktree))
        self.assertTrue((args.worktree / "fork/series.json").exists())
        self.assertIn(stack.README_MARKER, (args.worktree / "README.md").read_text())

    def test_untracked_source_is_rejected_before_replay(self):
        self.write("new_runtime.py", "print('new behavior')\n")
        args = self.args()
        with self.assertRaisesRegex(ValueError, "including new files"):
            stack.replay(self.data, args)
        self.assertFalse(args.worktree.exists())

    def test_committed_runtime_drift_is_rejected_before_replay(self):
        self.write("app.txt", "unexported committed behavior\n")
        self.commit("fix: unexported behavior")
        args = self.args()
        with self.assertRaises(subprocess.CalledProcessError):
            stack.replay(self.data, args)
        self.assertFalse(args.worktree.exists())

    def test_upstream_readme_edits_are_not_silently_discarded(self):
        self.write("README.md", "# Edited upstream text\n" + stack.README_MARKER + "- Feature\n")
        self.commit("docs: unexported upstream edit")
        with self.assertRaisesRegex(ValueError, "outside the personal appendix"):
            stack.replay(self.data, self.args())


if __name__ == "__main__":
    unittest.main()
