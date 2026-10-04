# Maintaining the Personal fork

The daemon/frontend keep upstream's layout. Personal behavior lives in a linear
Git stack, one feature per commit, followed by one maintenance commit. Do not
move daemon behavior into a fork-only wrapper or duplicate implementation files.

## Branches and source of truth

- `main`: GitHub's clean upstream branch. No personal work belongs here.
- `upstream/v0.13.3`: immutable starting point of the current tested stack.
- `personal/maintained`: initial maintained Personal stack. The fork's default
  branch identifies the current maintained source after later promotions.
- `personal/v0.13.1`: original published snapshot, retained without rewriting.

The initial stack was verified on upstream `99d4035e9` (`v0.13.3`), **not latest
upstream at reconstruction time**. The current exact base and ordered feature
commits are recorded in `series.json`, which is refreshed after each port.
The source files and feature commits are authoritative; `patches/*.patch` are
generated `git format-patch` exports. Never independently edit both source and
patch text. Re-export after changing or porting a feature. Checksums and an exact
Git-tree replay catch missing source, generated files, and stale exports.

Use `python3 fork/stack.py status` for the inventory and
`python3 fork/stack.py verify` for reproducibility. Python 3 and Git are the only
tool dependencies. Verification does not start the daemon or access `~/.ao`.

## Feature boundaries

| Patch | Purpose | Contribution status | Dependencies |
| --- | --- | --- | --- |
| `codex-protocol` | Runtime Codex capabilities, host-bound protocol snapshots, paginated history and revert | Candidate | None |
| `codex-report-delivery` | Native-history acceptance, guarded draft submission and retries | Candidate | None |
| `parallel-orchestrators` | Independent coordinator sessions, tabs, worker/report ownership, API and persistence | Candidate | Report delivery interface |
| `legacy-ownership-migration` | Repair databases created by the old Personal `0156` migration | Personal only | Parallel orchestrators |
| `worker-defaults` | Current project Worker settings and prompt guidance | Candidate | Parallel orchestrator interfaces |
| `installed-cli-checks` | Opt-in Codex/Antigravity/OpenCode protocol and launch checks | Candidate | Codex protocol and parallel CLI ID support |
| `personal-build` | Desktop revision label; no dependency version changes | Personal only | None |

"Candidate" means a separable implementation, not an approved or already
submitted upstream contribution. Some features still need their listed
dependencies. Further extraction should happen on the contribution branch,
with its own tests, rather than making the runtime switch between patch systems.

## Updating upstream safely

Configure remotes once (use your existing upstream remote if already configured):

```bash
git remote add upstream https://github.com/Untrivial-ai/agent-orchestrator.git
git remote add personal https://github.com/inevitabledawn/agent-orchestrator-personal.git
git fetch upstream main
python3 fork/stack.py replay --onto upstream/main \
  --branch personal/update-YYYY-MM-DD \
  --worktree ../agent-orchestrator-personal-update
```

`replay` creates a new branch/worktree and applies each feature with `git am
--3way`. It never rebases, force-pushes, overwrites an existing branch/directory,
or changes the invoking checkout. A successful replay refreshes its patch
bundle and appends the fork section to the new upstream README. The maintenance
layer remains uncommitted until you review it. After a text conflict, the new
checkout is retained: resolve it, `git add` only the resolved files, run `git am
--continue`, and apply the remaining patch files from the ordered manifest with
`git am --3way`. Then regenerate the maintenance layer from the old checkout:

```bash
python3 fork/stack.py finalize --base upstream/main \
  --worktree ../agent-orchestrator-personal-update
```

Pin the intended upstream commit SHA instead of a moving `upstream/main` if you
fetch again during conflict resolution. Do not skip a patch just to get a clean
application: check whether upstream absorbed its behavior, and update the
manifest/dependencies deliberately if retiring it. `export` requires one commit
per recorded feature, in order, with its recorded subject:

```bash
python3 fork/stack.py export --base UPSTREAM_SHA --tip FEATURE_TIP_SHA \
  --output ../personal-patch-bundle
```

The output must be a new directory. Copy the reviewed new `series.json` and
`patches/` into the matching maintained source branch, then commit maintenance.
When developing, keep feature changes with their owning commit and exclude the
maintenance commit from the export range. Local fixup/squash work belongs on a
new unpublished branch; never rewrite the published maintained branch.

### SQLite migration collisions

Fresh upstream has already assigned `0171` to a different migration. The current
Personal base legitimately uses `0171_parallel_orchestrator_ownership.sql`.
The tool rejects a newer base that owns the same numeric prefix **before** it
creates a worktree. This prevents a silent schema failure even when text patches
would merge cleanly.

When porting to that upstream, allocate a free migration number in the new
integration branch. Update the new ownership migration, shipped-version test,
legacy repair mapping/tests, and generated SQL together. Do not edit migrations
already merged in upstream, and do not treat existing Personal databases as
fresh installs. Preserve both the old `0156` Personal mapping and the installed
`0171` Personal mapping; a future migration repair must explicitly recognize
both before applying upstream's different `0171`. Run migration tests against
scratch databases/copies only. Automatic semantic conflict resolution is not
implemented.

## Verification before promoting an update

Install each checkout's own dependencies; do not symlink another checkout's
`node_modules`. Shared UI dependencies are required by frontend typechecking:

```bash
npm ci --prefix packages/product-ui
npm ci --prefix frontend
python3 -m unittest discover -s fork -p 'test_*.py'
cd backend
go test ./...
go build ./...
go vet ./...
cd ../frontend
npm run typecheck
npm test
CI=1 AO_E2E_PORT=5199 npx playwright test e2e/orchestrator-session-tabs.spec.ts
```

If SQL/API sources changed, run the repository's pinned `npm run sqlc` and
`npm run api` generators and inspect their diffs. Run
`bash test/check-local-cli-compatibility.sh` after provider CLI updates; it performs
handshakes/flag reads without model prompts. Renderer e2e uses a fake daemon
bridge: it verifies navigation and tabs, not installed Electron or live provider
behavior. A packaged desktop promotion additionally needs the repository's
isolated desktop lab and scratch `AO_DATA_DIR`; this tooling does not install or
replace the user's running app.

After passing checks, commit `fork/`, the README appendix, and maintenance docs
in the new checkout. Run `python3 fork/stack.py verify`. Push the new branch
normally, review the changes/range-diff, then choose it as the fork's default
branch. Keep the prior maintained branch for rollback. No force push is needed.

## Preparing a contribution

```bash
python3 fork/stack.py contribution --feature codex-protocol \
  --branch contribution/codex-protocol \
  --worktree ../agent-orchestrator-contribution
```

This creates a branch from the pinned clean base and applies the requested
candidate plus declared dependencies. `--onto upstream/main` selects a newer
base and runs the same migration guard. Personal build labeling, old Personal
database repair, and fork maintenance files never enter the candidate branch.
No PR is created. Port to current upstream, run that repository's complete
required CI checks, and inspect all generated artifacts before submitting.

## Initial preservation evidence

The stack was reconstructed from the canonical upstream-compatible Personal
source on 2026-10-04. Runtime source equals that preserved checkout, except that
an accidental `type-fest` lockfile version substitution was corrected back to
upstream `0.13.1`; only the root desktop version is labeled Personal. Original
dirty checkouts and the original GitHub branch were retained. This is source
organization and replay verification; it does not replace the installed app.
