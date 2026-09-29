package codex

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
	"github.com/aoagents/agent-orchestrator/backend/internal/ports"
)

func TestDetectTerminalActivity(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   domain.ActivityState
		ok     bool
	}{
		{
			name:   "idle composer",
			output: "\x1b[1m›\x1b[0m \x1b[2mWrite tests for @filename\x1b[0m\n\ngpt-5.6-sol low · ~/project\n",
			want:   domain.ActivityIdle,
			ok:     true,
		},
		{
			name:   "working composer",
			output: "• Working (2m 10s • esc to interrupt)\n› Add tests\n\ngpt-5.6-sol low · ~/project\n",
		},
		{
			name:   "approval picker",
			output: "› 1. Approve once\n  2. Deny\nPress enter to confirm or esc to go back\n",
		},
		{
			name:   "assistant text",
			output: "The symbol is:\n›\nnot a composer footer\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := (&Plugin{}).DetectTerminalActivity(tt.output)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("DetectTerminalActivity() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestInspectTerminalSurfaceSeparatesCodexWorkFromComposer(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantWork   ports.TerminalSurfaceWorkState
		wantEditor ports.TerminalComposerState
	}{
		{
			name:       "idle empty composer",
			output:     "\x1b[1m›\x1b[0m \x1b[2mWrite tests for @filename\x1b[0m\n\ngpt-5.6-sol low · ~/project\n",
			wantWork:   ports.TerminalSurfaceWorkIdle,
			wantEditor: ports.TerminalComposerEmpty,
		},
		{
			name:       "idle empty composer when constrained viewport hides footer",
			output:     "\x1b[2m• \x1b[0mE2E_ROUNDTRIP_TWO\n\n\n\x1b[1m›\x1b[0m\n",
			wantWork:   ports.TerminalSurfaceWorkIdle,
			wantEditor: ports.TerminalComposerEmpty,
		},
		{
			name:       "plain transcript prompt without footer is not current chrome",
			output:     "The response ended with an example:\n›\n",
			wantWork:   ports.TerminalSurfaceWorkUnknown,
			wantEditor: ports.TerminalComposerEmpty,
		},
		{
			name:       "active empty composer when constrained viewport hides footer",
			output:     "\x1b[2m• Working (4s • esc to interrupt)\x1b[0m\n\n\x1b[1m›\x1b[0m\n",
			wantWork:   ports.TerminalSurfaceWorkActive,
			wantEditor: ports.TerminalComposerEmpty,
		},
		{
			name:       "idle draft",
			output:     "› Keep this draft\n\ngpt-5.6-sol low · ~/project\n",
			wantWork:   ports.TerminalSurfaceWorkIdle,
			wantEditor: ports.TerminalComposerDraft,
		},
		{
			name:       "active wording inside draft is not current chrome",
			output:     "› Quote esc to interrupt here\n\ngpt-5.6-sol low · ~/project\n",
			wantWork:   ports.TerminalSurfaceWorkIdle,
			wantEditor: ports.TerminalComposerDraft,
		},
		{
			name:       "active empty composer",
			output:     "• Working (2m 10s • esc to interrupt)\n› \x1b[2mAdd tests\x1b[0m\n\ngpt-5.6-sol low · ~/project\n",
			wantWork:   ports.TerminalSurfaceWorkActive,
			wantEditor: ports.TerminalComposerEmpty,
		},
		{
			name: "old active row in transcript is not current chrome",
			output: "• Working (2m 10s • esc to interrupt)\nThe work finished.\n" +
				"› \x1b[2mAdd tests\x1b[0m\n\ngpt-5.6-sol low · ~/project\n",
			wantWork:   ports.TerminalSurfaceWorkIdle,
			wantEditor: ports.TerminalComposerEmpty,
		},
		{
			name:       "approval picker",
			output:     "› 1. Approve once\n  2. Deny\nPress enter to confirm or esc to go back\n",
			wantWork:   ports.TerminalSurfaceWorkWaitingInput,
			wantEditor: ports.TerminalComposerUnknown,
		},
		{
			name: "approval picker with normal footer still visible",
			output: "Run this command?\n› 1. Approve once\n  2. Deny\n" +
				"Press enter to confirm or esc to go back\n\ngpt-5.6-sol low · ~/project\n",
			wantWork:   ports.TerminalSurfaceWorkWaitingInput,
			wantEditor: ports.TerminalComposerUnknown,
		},
		{
			name:       "approval picker with a non-first selected option",
			output:     "Run this command?\n  1. Approve once\n› 2. Deny\nPress enter to confirm or esc to go back\n",
			wantWork:   ports.TerminalSurfaceWorkWaitingInput,
			wantEditor: ports.TerminalComposerUnknown,
		},
		{
			name: "completed approval picker above the current composer is idle",
			output: "Run this command?\n› 1. Approve once\n  2. Deny\nPress enter to confirm or esc to go back\n" +
				"› \x1b[2mAdd tests\x1b[0m\n\ngpt-5.6-sol low · ~/project\n",
			wantWork:   ports.TerminalSurfaceWorkIdle,
			wantEditor: ports.TerminalComposerEmpty,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&Plugin{}).InspectTerminalSurface(tt.output)
			if got.Work != tt.wantWork || got.Composer != tt.wantEditor {
				t.Fatalf("InspectTerminalSurface() = %+v, want work=%v composer=%v", got, tt.wantWork, tt.wantEditor)
			}
		})
	}
}

func TestReportDraftVisibleRequiresCurrentNormalComposerAndMatchingID(t *testing.T) {
	p := &Plugin{}
	id := "report-batch:one"
	draft := "› <ao-report-delivery id=\"report-batch:one\">\nReport text\n</ao-report-delivery>\n<ao-report-delivery-end id=\"report-batch:one\"/>\n\ngpt-5.6-sol low · ~/project\n"
	if !p.ReportDraftVisible(draft, id) {
		t.Fatal("matching report in normal composer must be visible")
	}
	for name, output := range map[string]string{
		"other delivery":    strings.ReplaceAll(draft, "report-batch:one", "report-batch:two"),
		"approval":          "› 1. Approve once\n  2. Deny\nPress enter to confirm or esc to go back\n",
		"modal after draft": draft[:strings.Index(draft, "\n\ngpt-")] + "\nApproval required\nPress enter to confirm\n\ngpt-5.6-sol low · ~/project\n",
		"transcript":        "› <ao-report-delivery id=\"report-batch:one\">\nReport text\n",
		"empty":             "› \x1b[2mAdd tests\x1b[0m\n\ngpt-5.6-sol low · ~/project\n",
	} {
		t.Run(name, func(t *testing.T) {
			if p.ReportDraftVisible(output, id) {
				t.Fatal("must not treat ambiguous surface as a report draft")
			}
		})
	}
}

func TestReportDraftVisibleWhenLargeComposerFillsViewport(t *testing.T) {
	var output strings.Builder
	output.WriteString("\x1b[1m›\x1b[0m status line 091\n")
	for i := 92; i < 120; i++ {
		fmt.Fprintf(&output, "  status line %03d\n", i)
	}
	output.WriteString("  <ao-report-delivery-end id=\"report-batch:large\"/>\n\n")
	output.WriteString("  GPT-6-Sol medium · ~\n  tab to queue message\n")
	if !New().ReportDraftVisible(output.String(), "report-batch:large") {
		t.Fatal("visible delivery marker in a long normal composer must permit Enter-only rescue")
	}
}

func TestInspectTerminalSurfaceOnlyProvesAnUnstartedConversationOnInitialFrame(t *testing.T) {
	header := "╭────────────────────────╮\n│ >_ OpenAI Codex (v0.147.0) │\n╰────────────────────────╯\n\nTip: Try the Desktop app.\n"
	footer := "\n\ngpt-5.6-sol low · ~/project\n"
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "initial empty composer",
			output: header + "\n\x1b[1m›\x1b[0m \x1b[2mSummarize recent commits\x1b[0m" + footer,
			want:   true,
		},
		{
			name:   "initial composer with unsent draft",
			output: header + "\n\x1b[1m›\x1b[0m Keep this draft" + footer,
			want:   true,
		},
		{
			name: "completed turn",
			output: header + "\n› Say hello\n\n• Hello\n\n" +
				"\x1b[1m›\x1b[0m \x1b[2mSummarize recent commits\x1b[0m" + footer,
		},
		{
			name:   "partial frame without provider header",
			output: "\x1b[1m›\x1b[0m \x1b[2mSummarize recent commits\x1b[0m" + footer,
		},
		{
			name:   "active turn",
			output: header + "\n• Working (4s • esc to interrupt)\n\x1b[1m›\x1b[0m" + footer,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := (&Plugin{}).InspectTerminalSurface(tt.output)
			if got.NativeConversationNotStarted != tt.want {
				t.Fatalf("NativeConversationNotStarted = %v, want %v; observation=%+v", got.NativeConversationNotStarted, tt.want, got)
			}
		})
	}
}
