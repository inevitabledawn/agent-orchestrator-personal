package codex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportAcceptedInTranscriptRequiresNativeUserMessage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	rows := []string{
		`{"type":"session_meta","payload":{"id":"native-1"}}`,
		`{"type":"event_msg","payload":{"type":"agent_message","message":"<ao-report-delivery id=\"report-batch:one\">"}}`,
		`{"type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"<ao-report-delivery id=\"report-batch:one\">"}]}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"<ao-report-delivery id=\"report-batch:other\">"}}`,
	}
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	join := func(rows []string) string {
		var content string
		for _, row := range rows {
			content += row + "\n"
		}
		return content
	}
	write(join(rows))
	p := New()
	if accepted, err := p.ReportAcceptedInTranscript(context.Background(), path, "report-batch:one"); err != nil || accepted {
		t.Fatalf("non-user occurrence accepted=%v err=%v", accepted, err)
	}
	write(strings.TrimSuffix(join(rows), "\n"))
	if _, err := p.ReportAcceptedInTranscript(context.Background(), path, "report-batch:one"); err == nil {
		t.Fatal("incomplete final record must not prove absence of a report")
	}
	rows = append(rows, `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<ao-report-delivery id=\"report-batch:one\">\nReport"}]}}`)
	write(join(rows))
	if accepted, err := p.ReportAcceptedInTranscript(context.Background(), path, "report-batch:one"); err != nil || !accepted {
		t.Fatalf("native user message accepted=%v err=%v", accepted, err)
	}
}
