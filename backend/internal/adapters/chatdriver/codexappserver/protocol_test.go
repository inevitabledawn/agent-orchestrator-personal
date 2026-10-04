package codexappserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/adapters/chatdriver/persistenthost"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

// Removing optional provider APIs must not prevent opening a working conversation.
func TestInstalledProtocolDisablesMissingOptionalFeatures(t *testing.T) {
	d, fixture := protocolFixtureDriver(t)
	caps, err := d.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if caps.Has(ports.ChatCapabilityCompaction) || caps.Has(ports.ChatCapabilityRollback) || caps.Has(ports.ChatCapabilitySteer) {
		t.Fatalf("advertised removed optional APIs: %v", caps)
	}
	if missing := ports.MissingProductionCapabilities(caps); len(missing) != 0 {
		t.Fatalf("core capabilities missing: %v", missing)
	}
	conv, err := d.Start(context.Background(), ports.ChatStartConfig{WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conv.Close() }()
	if conv.Capabilities().Has(ports.ChatCapabilityCompaction) {
		t.Fatal("conversation ignored the installed protocol")
	}
	compactCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := conv.(*conversation).Compact(compactCtx); !errors.Is(err, ports.ErrChatUnsupported) {
		t.Fatalf("Compact = %v, want unsupported", err)
	}
	count, err := os.ReadFile(filepath.Join(fixture, "exports"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(count), "export") != 1 {
		t.Fatalf("schema should be cached, exports = %s", count)
	}
}

func TestInstalledProtocolRefreshesAfterCodexUpdate(t *testing.T) {
	d, fixture := protocolFixtureDriver(t)
	if _, err := d.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
	addFixtureMethod(t, fixture, "ClientRequest", "thread/compact/start")
	if err := os.WriteFile(filepath.Join(fixture, "version"), []byte("codex-cli 0.160.0\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	caps, err := d.Probe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !caps.Has(ports.ChatCapabilityCompaction) {
		t.Fatal("updated Codex schema was not picked up")
	}
}

func TestInstalledProtocolRejectsRemovedCoreMethodBeforeSpawn(t *testing.T) {
	d, fixture := protocolFixtureDriver(t)
	if err := os.WriteFile(filepath.Join(fixture, "ServerNotification.json"), []byte(`{"oneOf":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Start(context.Background(), ports.ChatStartConfig{WorkspacePath: t.TempDir()}); !errors.Is(err, ports.ErrChatDriverIncompatible) {
		t.Fatalf("Start = %v, want incompatible", err)
	}
}

func TestInstalledProtocolRejectsUnknownApprovalBeforeSpawn(t *testing.T) {
	d, fixture := protocolFixtureDriver(t)
	addFixtureMethod(t, fixture, "ServerRequest", "item/newDangerousTool/requestApproval")
	if _, err := d.Probe(context.Background()); !errors.Is(err, ports.ErrChatDriverIncompatible) {
		t.Fatalf("Probe = %v, want incompatible", err)
	}
}

func TestInstalledProtocolMalformedSchemaFailsClosed(t *testing.T) {
	d, fixture := protocolFixtureDriver(t)
	if err := os.WriteFile(filepath.Join(fixture, "ClientRequest.json"), []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Probe(context.Background()); !errors.Is(err, ports.ErrChatDriverIncompatible) {
		t.Fatalf("Probe = %v, want incompatible", err)
	}
}

func protocolFixtureDriver(t *testing.T) (*Driver, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX executable fixture")
	}
	fixture := t.TempDir()
	t.Setenv("AO_DATA_DIR", t.TempDir())
	methods := map[string][]string{
		"ClientRequest":      {"initialize", "thread/start", "thread/resume", "turn/start", "turn/interrupt", "model/list", "thread/read"},
		"ClientNotification": {"initialized"},
		"ServerNotification": {"turn/started", "turn/completed", "item/started", "item/completed", "item/agentMessage/delta"},
		"ServerRequest":      {"item/commandExecution/requestApproval", "item/fileChange/requestApproval", "item/permissions/requestApproval", "item/tool/requestUserInput"},
	}
	for direction, names := range methods {
		for _, name := range names {
			addFixtureMethod(t, fixture, direction, name)
		}
	}
	if err := os.WriteFile(filepath.Join(fixture, "version"), []byte("codex-cli 0.159.2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(fixture, "codex")
	script := "#!/bin/sh\nbase=$(dirname \"$0\")\nif [ \"$1\" = --version ]; then cat \"$base/version\"; exit; fi\nif [ \"$2\" = generate-json-schema ]; then\n echo export >> \"$base/exports\"\n cp \"$base/\"*.json \"$4/\"\n exit\nfi\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	d := New(fakePlugin{bin: bin}, slog.New(slog.DiscardHandler))
	d.spawn = func(ctx context.Context, bin, workdir string, env []string) (*process, error) {
		fake, _ := newTestDriver(t)
		return fake.spawn(ctx, bin, workdir, env)
	}
	d.persistent = false
	return d, fixture
}

func addFixtureMethod(t *testing.T, dir, direction, method string) {
	t.Helper()
	path := filepath.Join(dir, direction+".json")
	var doc struct {
		OneOf []json.RawMessage `json:"oneOf"`
	}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
	}
	arm, err := json.Marshal(map[string]any{"properties": map[string]any{"method": map[string]any{"enum": []string{method}}}})
	if err != nil {
		t.Fatal(err)
	}
	doc.OneOf = append(doc.OneOf, arm)
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestProtocolMethodsFollowLocalReferencesAndConstants(t *testing.T) {
	dir := t.TempDir()
	for _, direction := range []string{"ClientRequest", "ClientNotification", "ServerRequest", "ServerNotification"} {
		if err := os.WriteFile(filepath.Join(dir, direction+".json"), []byte(`{"oneOf":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw := `{"oneOf":[{"$ref":"#/$defs/request"}],"$defs":{"request":{"properties":{"method":{"const":"initialize"}}}}}`
	if err := os.WriteFile(filepath.Join(dir, "ClientRequest.json"), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	methods, err := readProtocolMethods(dir)
	if err != nil {
		t.Fatal(err)
	}
	if methods["initialize"] != "ClientRequest" {
		t.Fatalf("methods = %v", methods)
	}
}

func TestMalformedTurnLifecycleProducesCompatibilityError(t *testing.T) {
	for _, method := range []string{"turn/started", "turn/completed"} {
		for _, params := range []string{`{"turn":42}`, `{"threadId":"th1","turn":{"identifier":"changed"}}`, `{"threadId":"th1","turn":{"id":"tu1"}}`} {
			events := normalizeNotification(notification{Method: method, Params: json.RawMessage(params)}, testNow)
			if len(events) != 1 || events[0].Kind != ports.ChatEventError || !errors.Is(events[0].Err, ports.ErrChatDriverIncompatible) {
				t.Fatalf("%s malformed lifecycle %s -> %+v, want explicit compatibility error", method, params, events)
			}
		}
	}
}

func TestLifecycleIgnoresChangesToUnreadProviderFields(t *testing.T) {
	event := normalizeOne(t, "turn/completed", `{"threadId":"th1","newField":{"arbitrary":true},"turn":{"id":"tu1","status":"completed","items":{"newPaginationShape":true}}}`)
	if event.Kind != ports.ChatEventTurnCompleted || event.ProviderTurnID != "tu1" {
		t.Fatalf("compatible lifecycle extension was rejected: %+v", event)
	}
}

func TestMalformedCompletionSurfacesOnLiveEventStream(t *testing.T) {
	d, srv := newTestDriver(t)
	conv, err := d.Start(context.Background(), ports.ChatStartConfig{WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conv.Close() }()
	srv.push(`{"method":"turn/completed","params":{"threadId":"thread-1","turn":{"identifier":"renamed"}}}`)
	event := nextEvent(t, conv.Events(), ports.ChatEventError)
	if !errors.Is(event.Err, ports.ErrChatDriverIncompatible) {
		t.Fatalf("error = %v", event.Err)
	}
}

func TestPersistentReconnectKeepsOriginalProtocolAfterCLIUpdate(t *testing.T) {
	d, fixture := protocolFixtureDriver(t)
	dataDir := t.TempDir()
	d.persistent = true
	d.connectHost = func(ctx context.Context, cfg persistenthost.Config) (*persistenthost.Transport, error) {
		proc, err := d.spawn(ctx, cfg.Argv[0], cfg.Workdir, cfg.Env)
		if err != nil {
			return nil, err
		}
		return &persistenthost.Transport{Stdin: proc.stdin, Stdout: proc.stdout, InstanceID: "provider-instance-1"}, nil
	}
	conv, err := d.Start(context.Background(), ports.ChatStartConfig{SessionID: "persistent-protocol", DataDir: dataDir, WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	_ = conv.Close()
	// The installed executable is now broken, while the prior process survives.
	if err := os.WriteFile(filepath.Join(fixture, "version"), []byte("changed provider\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "ClientRequest.json"), []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	replacement := New(fakePlugin{bin: filepath.Join(fixture, "codex")}, nil)
	replacement.connectHost = func(ctx context.Context, cfg persistenthost.Config) (*persistenthost.Transport, error) {
		proc, err := d.spawn(ctx, cfg.Argv[0], cfg.Workdir, cfg.Env)
		if err != nil {
			return nil, err
		}
		return &persistenthost.Transport{Stdin: proc.stdin, Stdout: proc.stdout, Reconnected: true, InstanceID: "provider-instance-1"}, nil
	}
	resumed, err := replacement.Resume(context.Background(), ports.ChatResumeConfig{SessionID: "persistent-protocol", DataDir: dataDir, WorkspacePath: t.TempDir(), ProviderConversationID: "thread-1"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resumed.Close() }()
	if resumed.Capabilities().Has(ports.ChatCapabilityCompaction) {
		t.Fatal("reconnect lost the original protocol snapshot")
	}
}

func TestReconnectWithoutTrustedSnapshotPreservesCoreSession(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"legacy", ""},
		{"corrupt", `{`},
		{"prior host", `{"instanceId":"prior-host","methods":{"thread/compact/start":"ClientRequest"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, srv := newTestDriver(t)
			dataDir := t.TempDir()
			dir := filepath.Join(dataDir, "chat-hosts", "legacy-host")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if tc.content != "" {
				if err := os.WriteFile(filepath.Join(dir, "codex-protocol.json"), []byte(tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			proc, err := d.spawn(context.Background(), "codex", t.TempDir(), nil)
			if err != nil {
				t.Fatal(err)
			}
			d.persistent = true
			d.connectHost = func(context.Context, persistenthost.Config) (*persistenthost.Transport, error) {
				return &persistenthost.Transport{Stdin: proc.stdin, Stdout: proc.stdout, Reconnected: true, InstanceID: "live-host"}, nil
			}
			d.schemaProbe = func(context.Context, string) (map[string]string, error) {
				t.Fatal("reconnect must not inspect the new executable")
				return nil, nil
			}
			conv, err := d.Resume(context.Background(), ports.ChatResumeConfig{SessionID: "legacy-host", DataDir: dataDir, WorkspacePath: t.TempDir(), ProviderConversationID: "thread-1"})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conv.Close() }()
			caps := conv.Capabilities()
			if caps.Has(ports.ChatCapabilityCompaction) || caps.Has(ports.ChatCapabilityRollback) {
				t.Fatalf("unverified optional APIs advertised: %v", caps)
			}
			if missing := ports.MissingProductionCapabilities(caps); len(missing) != 0 {
				t.Fatalf("live core session unavailable: %v", missing)
			}
			if _, err := conv.SendTurn(context.Background(), ports.ChatUserMessage{Text: "continue"}); err != nil {
				t.Fatal(err)
			}
			if srv.sentMethod("initialize") || srv.sentMethod("thread/resume") {
				t.Fatal("reattach repeated native initialization")
			}
		})
	}
}
