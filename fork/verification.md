# Initial stack verification

Verified on 2026-10-04 against the pinned upstream v0.13.3 base. This receipt
describes the initial reconstruction; rerun checks after future ports rather
than treating it as evidence for a new base.

- Preservation: all canonical Personal runtime/source files match the saved
  compatible checkout. The only configuration correction restores the
  `type-fest` lockfile version to upstream `0.13.1`; desktop labeling remains
  `0.13.3-personal.1`. Both pre-existing dirty checkouts were untouched.
- Backend: full `go test ./...`, `go build ./...`, `go vet ./...`, and pinned
  golangci-lint `v2.13.2` passed (zero lint issues).
- Frontend: typecheck passed; 360 test files passed, with 5,761 tests passed
  and seven skipped. Browser navigation/session-tab e2e passed (one test), and
  its generated screenshot was inspected. This is renderer verification with
  the fake bridge, not an installed Electron/provider session test.
- Installed providers: the read-only Codex schema/initialize/model inventory,
  OpenCode ACP initialize, and Antigravity launch-flag checks passed. These
  calls send no model prompts and do not verify paid model turns.
- Patch tooling: 14 real-Git tests passed, including migration collision,
  conflict retention, stale/untracked source rejection, export ordering,
  maintenance finalization, and contribution dependency filtering.
- Exact stack replay: all seven exported patches applied to pristine upstream;
  the resulting Git tree exactly matched the functional feature-stack tip.
  A real full replay also produced a refreshed maintenance bundle in an isolated
  checkout without changing the invoking source.
- Contribution isolation: the standalone Codex report-delivery candidate applied
  without the protocol/ownership/local-build layers; its affected Go suites passed.
- Latest-upstream guard: replay onto `b7f64053a` was rejected before creating
  a worktree because upstream already owns migration number `0171`. The stack
  has not been ported to that commit.

No production AO data, installed app, release, or upstream PR was changed.
Full packaged desktop builds, race suites, and every platform's CI job were
not run for this source organization task. An actual upstream PR or desktop
promotion must run its documented complete validation separately.
