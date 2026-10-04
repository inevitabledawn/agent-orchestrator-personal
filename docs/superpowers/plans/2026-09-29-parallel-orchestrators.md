# Parallel Orchestrators Implementation Plan

**Goal:** Run independent orchestrator Chat sessions in one project, with durable worker ownership and report delivery to the correct orchestrator.

**Architecture:** Keep the existing project orchestrator and API ensure/restart behavior for compatibility. Add an explicit parallel spawn mode. Parallel orchestrators use unique worktree branches and session scoped conversations. Workers persist their orchestrator owner; prompts and report delivery use that owner after daemon restart. Legacy workers without an owner continue to use the legacy project orchestrator.

**Tech Stack:** Go daemon, SQLite/sqlc, generated OpenAPI TypeScript client, React/Electron.

**Spec:** User request in this conversation, 2026-09-29: multiple simultaneous orchestrators for one project, each communicating only with its own workers, with tabs in the existing strip.

## Global Constraints

- Do not change the installed app or live `~/.ao` data during development.
- Preserve the existing uncommitted TitlebarNav edits.
- Preserve current ensure and clean replacement semantics for legacy API callers.
- Do not edit generated sqlc or OpenAPI files by hand; regenerate from source.
- Do not commit or publish.

## Review Focus

- A worker created by orchestrator A stays with A after B is created and after daemon restart.
- Reports from A's workers never appear in B's automatic batch or piggyback.
- A terminated owner never causes its workers' reports to be rerouted to a different active owner.
- Two parallel Chat sessions have separate conversation records and worktree branches.
- Existing project conversation/history and ownerless workers keep legacy behavior.

## Tasks

### 1. Durable session ownership and isolated orchestrator identity

- Add a migration for worker `orchestrator_session_id` and orchestrator `parallel_orchestrator` facts on `sessions`.
- Regenerate sqlc and map both fields through session persistence. Validate a supplied worker parent as an active same-project orchestrator before creating the worker.
- Make explicit parallel orchestrator spawns use a unique branch and a session scoped Chat conversation; preserve project scoped legacy conversation behavior.
- Test persistence round trips, invalid parent rejection, two branch names, and two separate Chat histories.

### 2. Bound worker creation and instructions

- Pass the orchestrator ID through `ao spawn` and the UI delegate route into worker creation; persist the owner.
- Build and restore worker prompts using the persisted owner. For old ownerless workers, select only the legacy project orchestrator.
- Test two orchestrators spawning workers, restore prompts, and cross-project rejection.

### 3. Bound report delivery

- Scope report claims by the owner of each report's worker, while leaving the project report view unchanged.
- Route automatic delivery and Chat piggyback by owner. Never retarget a report from an owner that has terminated.
- Test two workers and two orchestrators for automatic, urgent, retry, and piggyback paths.

### 4. API and GUI

- Add `parallel` to the orchestrator create request, reject incompatible `clean && parallel`, and regenerate OpenAPI types.
- Wire the new session tab action to parallel spawn. Keep existing session navigation and archived history.
- Test the controller, typed client, tab action, and real GUI against isolated scratch data.

### 5. Verification

- Run focused Go/React tests, Go build, frontend typecheck/build, and an isolated two-orchestrator smoke test including worker messages and reports.
- Review final diff and leave the installed app untouched until the concurrent run is proven safe.
