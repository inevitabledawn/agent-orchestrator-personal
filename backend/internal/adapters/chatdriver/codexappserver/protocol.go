package codexappserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
	aoprocess "github.com/aoagents/agent-orchestrator/backend/internal/process"
)

// protocolProfile is immutable after export. It describes supported AO behavior,
// not arbitrary new provider features whose semantics AO cannot infer from JSON.
type protocolProfile struct {
	methods map[string]string
	caps    ports.ChatCapabilities
}

var coreProtocolMethods = map[string][]string{
	"ClientRequest":      {"initialize", "thread/start", "thread/resume", "turn/start", "turn/interrupt", "model/list"},
	"ClientNotification": {"initialized"},
	"ServerNotification": {"turn/started", "turn/completed", "item/started", "item/completed", "item/agentMessage/delta"},
	"ServerRequest":      {"item/commandExecution/requestApproval", "item/fileChange/requestApproval", "item/permissions/requestApproval"},
}

var optionalProtocolMethods = map[ports.ChatCapability][]string{
	ports.ChatCapabilityHistory:     {"thread/read", "thread/turns/list"},
	ports.ChatCapabilityUsage:       {"thread/tokenUsage/updated"},
	ports.ChatCapabilityDiffs:       {"turn/diff/updated"},
	ports.ChatCapabilityPlans:       {"turn/plan/updated"},
	ports.ChatCapabilityInteractive: {"item/tool/requestUserInput"},
	ports.ChatCapabilityCompaction:  {"thread/compact/start"},
	ports.ChatCapabilityRollback:    {"thread/revert", "thread/read", "thread/turns/list"},
	ports.ChatCapabilityFork:        {"thread/fork"},
	ports.ChatCapabilityRename:      {"thread/name/set"},
	ports.ChatCapabilitySkills:      {"skills/list"},
	ports.ChatCapabilityRateLimits:  {"account/rateLimits/read"},
	ports.ChatCapabilityMCPReload:   {"config/mcpServer/reload", "mcpServerStatus/list"},
	ports.ChatCapabilitySteer:       {"turn/steer"},
}

func profileFromMethods(methods map[string]string) (*protocolProfile, error) {
	for direction, names := range coreProtocolMethods {
		for _, name := range names {
			if methods[name] != direction {
				return nil, fmt.Errorf("required Codex method %s is missing or has changed direction", name)
			}
		}
	}
	for name, direction := range methods {
		if direction == "ServerRequest" && strings.HasSuffix(name, "/requestApproval") {
			if _, handled := approvalMethods[name]; !handled {
				return nil, fmt.Errorf("codex requires unsupported approval %s", name)
			}
		}
	}
	caps := capabilities()
	for capability, names := range optionalProtocolMethods {
		for _, name := range names {
			if methods[name] != expectedProtocolDirection(name) {
				caps[capability] = false
				break
			}
		}
	}
	return &protocolProfile{methods: methods, caps: caps}, nil
}

func expectedProtocolDirection(name string) string {
	switch name {
	case "thread/tokenUsage/updated", "turn/diff/updated", "turn/plan/updated":
		return "ServerNotification"
	case "item/tool/requestUserInput":
		return "ServerRequest"
	default:
		return "ClientRequest"
	}
}

// Cache by executable identity AND reported version: npm shims can remain
// unchanged when their native child is upgraded. A failed export is never cached.
func (d *Driver) installedProtocol(ctx context.Context, bin string) (*protocolProfile, error) {
	// Explicitly injected pipe transports use the compiled baseline.
	if d.schemaProbe == nil {
		return nil, nil
	}
	probe := d.versionProbe
	if probe == nil {
		probe = installedCodexVersion
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	version, err := probe(probeCtx, bin)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("%w: read Codex version: %w", ports.ErrChatDriverIncompatible, err)
	}
	identity := bin + ":" + strings.TrimSpace(version)
	if resolved, err := filepath.EvalSymlinks(bin); err == nil {
		identity = resolved + ":" + strings.TrimSpace(version)
	}
	if info, err := os.Stat(bin); err == nil {
		identity += fmt.Sprintf(":%d:%d", info.Size(), info.ModTime().UnixNano())
	}
	d.protocolMu.Lock()
	defer d.protocolMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if d.protocolKey == identity && d.protocol != nil {
		return d.protocol, nil
	}
	exportCtx, exportCancel := context.WithTimeout(ctx, 15*time.Second)
	defer exportCancel()
	methods, err := d.schemaProbe(exportCtx, bin)
	if err != nil {
		return nil, fmt.Errorf("%w: export Codex protocol: %w", ports.ErrChatDriverIncompatible, err)
	}
	profile, err := profileFromMethods(methods)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ports.ErrChatDriverIncompatible, err)
	}
	d.protocolKey, d.protocol = identity, profile
	return profile, nil
}

func exportInstalledProtocol(ctx context.Context, bin string) (map[string]string, error) {
	base := os.Getenv("AO_DATA_DIR")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		base = filepath.Join(home, ".ao")
	}
	if err := os.MkdirAll(base, 0o700); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(base, "codex-protocol-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	cmd := aoprocess.CommandContext(ctx, bin, "app-server", "generate-json-schema", "--out", dir)
	cmd.Env = codexProcessEnv(ctx, bin, nil)
	// Do not include provider stderr: it can contain private configuration values.
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return readProtocolMethods(dir)
}

func readProtocolMethods(dir string) (map[string]string, error) {
	methods := map[string]string{}
	for _, direction := range []string{"ClientRequest", "ClientNotification", "ServerRequest", "ServerNotification"} {
		raw, err := os.ReadFile(filepath.Join(dir, direction+".json"))
		if err != nil {
			return nil, err
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("parse %s schema: %w", direction, err)
		}
		var walk func(json.RawMessage, int) error
		walk = func(raw json.RawMessage, depth int) error {
			if depth > 32 {
				return fmt.Errorf("%s schema reference depth exceeded", direction)
			}
			var node struct {
				Ref        string            `json:"$ref"`
				OneOf      []json.RawMessage `json:"oneOf"`
				AnyOf      []json.RawMessage `json:"anyOf"`
				Properties struct {
					Method struct {
						Enum  []string `json:"enum"`
						Const string   `json:"const"`
					} `json:"method"`
				} `json:"properties"`
			}
			if err := json.Unmarshal(raw, &node); err != nil {
				return err
			}
			if node.Ref != "" {
				parts := strings.Split(node.Ref, "/")
				if len(parts) != 3 || parts[0] != "#" || (parts[1] != "definitions" && parts[1] != "$defs") {
					return fmt.Errorf("unsupported %s schema reference", direction)
				}
				var defs map[string]json.RawMessage
				if err := json.Unmarshal(doc[parts[1]], &defs); err != nil {
					return err
				}
				name := strings.ReplaceAll(strings.ReplaceAll(parts[2], "~1", "/"), "~0", "~")
				target, ok := defs[name]
				if !ok {
					return fmt.Errorf("missing %s schema reference", direction)
				}
				return walk(target, depth+1)
			}
			names := node.Properties.Method.Enum
			if node.Properties.Method.Const != "" {
				names = append(names, node.Properties.Method.Const)
			}
			for _, name := range names {
				if previous, ok := methods[name]; ok && previous != direction {
					return fmt.Errorf("conflicting direction for Codex method %s", name)
				}
				methods[name] = direction
			}
			for _, arm := range append(node.OneOf, node.AnyOf...) {
				if err := walk(arm, depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		if err := walk(raw, 0); err != nil {
			return nil, err
		}
	}
	return methods, nil
}

func (c *conversation) applyProtocol(profile *protocolProfile) {
	if profile == nil {
		return
	}
	c.protocolCaps = profile.caps
	c.conn.protocolMethods = profile.methods
}

// Snapshot the provider process, not the currently installed executable. An old
// app-server can survive both CLI upgrades and daemon replacement.
type hostProtocolSnapshot struct {
	InstanceID string            `json:"instanceId"`
	Methods    map[string]string `json:"methods"`
}

func hostProtocolPath(dataDir, sessionID string) (string, error) {
	if !filepath.IsAbs(dataDir) || sessionID == "" || sessionID == "." || sessionID == ".." || filepath.Base(sessionID) != sessionID || strings.ContainsAny(sessionID, `/\\`) {
		return "", fmt.Errorf("invalid Codex protocol snapshot location")
	}
	return filepath.Join(dataDir, "chat-hosts", sessionID, "codex-protocol.json"), nil
}

func saveHostProtocol(dataDir, sessionID, instanceID string, profile *protocolProfile) error {
	if profile == nil || instanceID == "" {
		return nil
	}
	path, err := hostProtocolPath(dataDir, sessionID)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(hostProtocolSnapshot{InstanceID: instanceID, Methods: profile.methods})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".codex-protocol-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	_, writeErr := file.Write(raw)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}

func loadHostProtocol(dataDir, sessionID, instanceID string) (*protocolProfile, error) {
	if instanceID == "" {
		return nil, nil
	}
	path, err := hostProtocolPath(dataDir, sessionID)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	} // Hosts started before dynamic discovery have no snapshot.
	if err != nil {
		return nil, err
	}
	var snapshot hostProtocolSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("invalid saved Codex protocol snapshot")
	}
	if snapshot.InstanceID != instanceID {
		return nil, nil
	} // A prior process's schema must never be reused.
	return profileFromMethods(snapshot.Methods)
}

// Legacy hosts have no recorded schema. Keep their already-established core
// transport usable without assuming optional APIs from the current executable.
func coreOnlyProtocol() *protocolProfile {
	methods := map[string]string{}
	for direction, names := range coreProtocolMethods {
		for _, name := range names {
			methods[name] = direction
		}
	}
	caps := capabilities()
	for capability := range optionalProtocolMethods {
		caps[capability] = false
	}
	return &protocolProfile{methods: methods, caps: caps}
}
