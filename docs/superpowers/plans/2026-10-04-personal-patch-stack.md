# Personal Patch Stack Implementation Plan

> **For agentic workers:** Use native execution for this authorized restructuring; bounded read-only/source-splitting delegation is allowed. Preserve existing checkouts and published branches.

**Goal:** Keep implemented Personal features as reviewable patches over clean upstream and document them in README.
**Architecture:** Reconstruct the canonical compatible source as seven linear feature commits on pinned v0.13.3, followed by a maintenance layer. Export checksummed patches and replay into new worktrees without rewriting existing branches.
**Tech Stack:** Git, Python standard library, existing Go/Electron/React tests.
**Spec:** User request on 2026-10-04; detailed operational design in `fork/README.md`.

## Global Constraints

Preserve dirty source trees, installed app, production AO data, and original published branch. Keep upstream code layout and generated contracts. Personal migration repair and build labeling stay out of contribution branches. No automatic force push, migration renumbering, release, or app installation.

## Review Focus

- Shared report/ownership/defaults files must retain all source behavior after hunk splitting.
- A colliding SQLite number must fail before creating an integration checkout.
- Failed patch application must retain its conflict while preserving the invoking branch.
- Exports must include generated code and match the feature Git tree exactly.
- README contributions must describe implemented fork behavior without claiming upstream acceptance or installed-app verification.

## Tasks

- [x] Snapshot canonical source and reconstruct protocol, report, ownership, legacy database repair, Worker defaults, CLI checks, and build commits; compile/test each affected boundary.
- [x] Implement `fork/stack.py` with status, exact verification, isolated replay/export, conflict finalization, and dependency-aware contribution preparation.
- [x] Add real-Git regression tests for replay, checksums, migration collisions, runtime drift, existing destinations, and conflict preservation.
- [x] Document branch ownership, migration handling, update/contribution workflows, and implemented features at the end of README.
- [ ] Run full relevant source checks, exact source/patch preservation, and independent final review; publish the new branch without rewriting previous branches.
