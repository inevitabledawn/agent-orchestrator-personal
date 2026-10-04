package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestAgentProcessSupervisesDottedProjectSession(t *testing.T) {
	cfg := setConfigEnv(t)
	srv, capture := activityServer(t, http.StatusOK, `{"ok":true}`)
	writeRunFileFor(t, cfg, srv)
	out, _, err := executeCLI(t, Deps{ProcessAlive: func(int) bool { return true }}, "agent-process", "supervise", "--session", "agent-orchestrator-personal-v0.13.1-1", "--launch", "launch-1", "--", "printf", "agent-started")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "agent-started") {
		t.Fatalf("agent not started: %q", out)
	}
	if capture.path != "/api/v1/sessions/agent-orchestrator-personal-v0.13.1-1/activity" {
		t.Fatalf("exit report path = %q", capture.path)
	}
}

func TestSessionIDPathValidation(t *testing.T) {
	for _, id := range []string{"qa-1", "v0.13.1-1", "proj_name-1"} {
		if !sessionIDPattern.MatchString(id) {
			t.Errorf("valid session rejected: %q", id)
		}
	}
	for _, id := range []string{"", ".", "..", "a..b", ".hidden", "a/../b", "a?x=y", "a#b", "a%2fb", "a\\b", "a b"} {
		if sessionIDPattern.MatchString(id) {
			t.Errorf("unsafe session accepted: %q", id)
		}
	}
}
