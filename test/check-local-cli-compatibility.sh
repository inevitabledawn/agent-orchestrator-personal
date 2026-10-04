#!/usr/bin/env bash
# Read-only protocol/flag checks against installed CLIs. No agent prompts.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
for cli in codex agy opencode; do
  if ! command -v "$cli" >/dev/null 2>&1; then
    echo "Required CLI is unavailable: $cli" >&2
    exit 1
  fi
done
cd "$repo_root/backend"
AO_AGENT_CLI_COMPAT=1 go test -timeout 90s \
  ./internal/adapters/chatdriver/codexappserver \
  ./internal/adapters/chatdriver/opencodeacp \
  ./internal/adapters/agent/agy \
  -run 'TestInstalled|TestGeneratedProtocolMatchesTheInstalledProvider' -count=1 -v
