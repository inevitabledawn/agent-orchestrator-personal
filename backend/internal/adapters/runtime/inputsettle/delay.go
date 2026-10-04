// Package inputsettle sizes the pause between terminal paste and Enter.
package inputsettle

import (
	"strings"
	"time"
)

const (
	baseDelay = 300 * time.Millisecond
	maxDelay  = 1500 * time.Millisecond
)

// Delay leaves normal prompts at the established pause, while allowing long
// multiline reports more time to leave the TUI's input queue before Enter.
func Delay(message string) time.Duration {
	if message == "" {
		return 0
	}
	extraKB := 0
	if len(message) > 1024 {
		extraKB = (len(message) - 1) / 1024
	}
	extraLines := strings.Count(message, "\n") / 10
	delay := baseDelay + time.Duration(extraKB)*75*time.Millisecond + time.Duration(extraLines)*30*time.Millisecond
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}
