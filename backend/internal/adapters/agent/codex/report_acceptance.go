package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

// ReportAcceptedInTranscript checks Codex's own durable user-message records.
// It is a fallback when AO's UserPromptSubmit callback was lost. A mere text
// occurrence in a tool result or assistant reply is never acceptance evidence.
func (p *Plugin) ReportAcceptedInTranscript(ctx context.Context, path, deliveryID string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	start, err := file.Stat()
	if err != nil {
		return false, err
	}
	if start.Size() == 0 {
		return false, fmt.Errorf("Codex rollout is empty")
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	needle := []byte(deliveryID)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if !bytes.Contains(scanner.Bytes(), needle) {
			continue
		}
		var line struct {
			Type    string `json:"type"`
			Payload struct {
				Type    string `json:"type"`
				Role    string `json:"role"`
				Message string `json:"message"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			return false, fmt.Errorf("decode Codex rollout: %w", err)
		}
		if line.Type == "event_msg" && line.Payload.Type == "user_message" {
			if id, ok := domain.ReportDeliveryID(line.Payload.Message); ok && id == deliveryID {
				return true, nil
			}
		}
		if line.Type == "response_item" && line.Payload.Type == "message" && line.Payload.Role == "user" {
			for _, part := range line.Payload.Content {
				if part.Type == "input_text" {
					if id, ok := domain.ReportDeliveryID(part.Text); ok && id == deliveryID {
						return true, nil
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("scan Codex rollout: %w", err)
	}
	end, err := file.Stat()
	if err != nil {
		return false, err
	}
	if end.Size() != start.Size() {
		return false, fmt.Errorf("Codex rollout changed during inspection")
	}
	var last [1]byte
	if _, err := file.ReadAt(last[:], end.Size()-1); err != nil || last[0] != '\n' {
		return false, fmt.Errorf("Codex rollout has an incomplete final record")
	}
	return false, ctx.Err()
}
