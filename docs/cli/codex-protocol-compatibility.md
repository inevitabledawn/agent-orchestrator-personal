# Codex protocol compatibility

AO discovers the installed Codex app-server method surface at runtime using
`codex app-server generate-json-schema`. The CLI exports a schema specific to
its own version, as described in the [official app-server documentation](https://learn.chatgpt.com/docs/app-server#message-schema).

## Behavior after an update

- Schema discovery happens for a new provider connection, before initialization.
  The required streaming, thread lifecycle, interrupt, model inventory, and
  approval methods must retain their expected direction.
- Optional capabilities (history editing, compaction, steering, skills, MCP
  reload, and others) are advertised only when their methods are present.
  Unsupported outbound requests are refused locally rather than sent to Codex.
- New methods, notifications, JSON fields, and string enum values do not require
  regenerating the compiled baseline merely because they were added. Unknown
  server requests are refused; AO never invents an approval decision.
- Lifecycle decoding projects the fields AO actually reads. Changes to unused
  turn metadata cannot prevent completion. Malformed required lifecycle data
  produces an explicit incompatibility event instead of silently disappearing.
- A schema introducing a new approval request is rejected before a new
  conversation starts. Its meaning needs an explicit implementation.

The schema cache is in memory, keyed by resolved executable path, reported CLI
version, file size, and modification time. The version check also detects npm
updates that leave the launcher shim unchanged. Failed exports are not cached.
Subprocesses have deadlines; schema export output is not included in error logs.
Temporary schema files live under `AO_DATA_DIR` (default `~/.ao`) and are removed.

## Existing sessions

For a detached persistent provider host, AO stores an atomic, private
`chat-hosts/<session-id>/codex-protocol.json` snapshot. It contains method names
and the host instance identity (PID and start timestamp), without credentials
or conversation content. Daemon replacement reloads that matching snapshot and
does not inspect the newly installed CLI or repeat initialization.

A legacy host without a matching, readable snapshot remains attached with core
capabilities only. Optional APIs are disabled and a warning is logged. This
also avoids applying a stale snapshot from a different process. Ordinary live
reconnect skips native history import; a TUI history handoff has separate
admission checks.

## Limits and verification

This is dynamic capability discovery over a stable AO adapter, not automatic
implementation of arbitrary protocol semantics. Renamed required fields,
changed request semantics, new permissions, or removed core methods can still
require an adapter update. AO reports these failures explicitly; it does not
translate history mutations or permissions by guessing.

Generated Go DTOs remain a reviewed baseline for the fields AO understands.
Regenerate them when adopting a new behavior or a demonstrated field change,
not after every additive CLI release. The compatibility test checks required
AO behavior, while removed unused methods and new realtime entry points are
informational.

Run `bash test/check-local-cli-compatibility.sh` after updating a provider.
Regression tests additionally exercise optional API removal, cache invalidation,
malformed schemas, unknown approvals, unused lifecycle field changes, and
reconnection to an old provider after a CLI update. The live check initializes
Codex and reads model inventory without sending a model prompt.
