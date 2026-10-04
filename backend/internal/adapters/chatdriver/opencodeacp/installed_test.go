package opencodeacp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Initialize only: no session/new, auth request, or prompt is sent. Provider
// configuration and writes are confined to scratch XDG directories.
func TestInstalledOpenCodeACPInitialize(t *testing.T) {
	if os.Getenv("AO_AGENT_CLI_COMPAT") != "1" {
		t.Skip("set AO_AGENT_CLI_COMPAT=1 for installed CLI checks")
	}
	bin, err := exec.LookPath("opencode")
	if err != nil {
		t.Fatal("opencode is required for this compatibility check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	scratch := t.TempDir()
	cmd := exec.CommandContext(ctx, bin, "acp")
	cmd.Dir = scratch
	cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+filepath.Join(scratch, "config"), "XDG_DATA_HOME="+filepath.Join(scratch, "data"), "XDG_CACHE_HOME="+filepath.Join(scratch, "cache"), "XDG_STATE_HOME="+filepath.Join(scratch, "state"), `OPENCODE_CONFIG_CONTENT={"plugin":[],"mcp":{}}`)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	request := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": 1, "clientInfo": map[string]string{"name": "ao-compatibility-check", "version": "1"}, "clientCapabilities": map[string]any{}}}
	if err := json.NewEncoder(stdin).Encode(request); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(stdout)
	for {
		var reply struct {
			ID     *int `json:"id"`
			Result struct {
				ProtocolVersion int `json:"protocolVersion"`
			} `json:"result"`
			Error *struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		if err := decoder.Decode(&reply); err != nil {
			t.Fatalf("installed OpenCode ACP initialize did not return a valid response: %v", err)
		}
		if reply.ID == nil || *reply.ID != 1 {
			continue
		}
		if reply.Error != nil {
			t.Fatalf("installed OpenCode refused ACP initialize (code %d)", reply.Error.Code)
		}
		if reply.Result.ProtocolVersion != 1 {
			t.Fatalf("unsupported ACP version %d", reply.Result.ProtocolVersion)
		}
		break
	}
}
