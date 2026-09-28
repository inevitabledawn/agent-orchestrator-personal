package inputsettle

import (
	"strings"
	"testing"
	"time"
)

func TestDelayKeepsShortPromptsFastAndBoundsLargeReports(t *testing.T) {
	if got := Delay("short prompt"); got != 300*time.Millisecond {
		t.Fatalf("short prompt delay = %s", got)
	}
	large := strings.Repeat("report line with useful context\n", 100)
	if got := Delay(large); got <= 300*time.Millisecond || got > 1500*time.Millisecond {
		t.Fatalf("large report delay = %s, want adaptive bounded pause", got)
	}
	if got := Delay(strings.Repeat("x", 1<<20)); got != 1500*time.Millisecond {
		t.Fatalf("huge report delay = %s, want cap", got)
	}
}
