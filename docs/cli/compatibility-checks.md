# Local CLI compatibility checks

Run `bash test/check-local-cli-compatibility.sh` after updating Codex, Antigravity
CLI, or OpenCode. It requires all three CLIs on PATH and checks:

- Codex's installed protocol schema, initialize handshake, and model inventory.
- OpenCode's ACP initialize response with scratch XDG configuration/data/cache.
- Antigravity's advertised flags against AO's actual launch and restore commands.

These checks send no model prompts and create no AO sessions. They fail on a
missing binary, rejected/malformed handshake, missing launch flag, schema-export
failure, or removed required protocol method. Each provider subprocess has a deadline.
The ordinary backend suite also checks mocked transport errors, history editing,
unknown provider messages, resume, permissions, and generated protocol types.

The 2026-10-01 check passed against Codex 0.159.2, agy 1.2.14, and OpenCode 1.18.33.
This is a compatibility snapshot, not a guarantee for future provider versions.
Antigravity hook execution/authentication and paid model turns are outside this
read-only check. OpenCode initialize alone does not verify login or model access.

If Codex's protocol changed, regenerate its bindings with
`cd backend && go run ./cmd/gencodexproto`, inspect the diff, and rerun the Codex
adapter suite. Regeneration alone is not evidence that new behavior is supported.

One stress-test limitation was observed during validation: ACP SDK 0.13.5 has a
1024-notification queue, and `TestACPReplayLoadLargeHistoryThroughSDK` overflowed
it once while the backend and frontend full suites ran concurrently. Five
isolated repeats and the subsequent full backend suite passed. This is an
unresolved intermittent stress-test risk; no transport behavior was changed
without a reproducible production failure.

Review also confirmed an existing upstream limitation at `ec1c6122a`: sending a
user-authored message during a Chat/TUI interface transition queues its text and
client ID without the authorship flag, then drains it as an internal message.
This predates the personal integration and was not changed here. Correct repair
requires durable authorship and user-turn retry identity in the transition
outbox; it is separate from installed CLI protocol compatibility.

Codex now discovers capabilities from the installed schema at runtime. See
[codex-protocol-compatibility.md](codex-protocol-compatibility.md) for update,
cache, persistent-session, and semantic compatibility boundaries.
