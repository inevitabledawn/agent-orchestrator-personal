package daemon

import (
	"context"
	"testing"

	"github.com/aoagents/agent-orchestrator/backend/internal/domain"
)

type reportDeliverySession struct {
	sentID      domain.SessionID
	sentMessage string
	sentKey     string
	sentAttempt int64
	interrupted domain.SessionID
}

func (s *reportDeliverySession) SendSemanticAttempt(_ context.Context, id domain.SessionID, message, key string, attempt int64) error {
	s.sentID, s.sentMessage, s.sentKey, s.sentAttempt = id, message, key, attempt
	return nil
}

func (s *reportDeliverySession) InterruptTUI(_ context.Context, id domain.SessionID) error {
	s.interrupted = id
	return nil
}

type reportDeliveryStore struct{ rec domain.SessionRecord }

func (s reportDeliveryStore) GetSession(context.Context, domain.SessionID) (domain.SessionRecord, bool, error) {
	return s.rec, true, nil
}

func TestReportSemanticDeliveryUsesAcceptedTUIBoundary(t *testing.T) {
	sessions := &reportDeliverySession{}
	delivery := reportSemanticDelivery{
		sessions: sessions,
		store: reportDeliveryStore{rec: domain.SessionRecord{
			ID: "ao-1", Mode: domain.SessionModeTUI,
		}},
	}
	if err := delivery.Submit(context.Background(), "ao-1", "report context", "report-batch:abc123", 1); err != nil {
		t.Fatal(err)
	}
	if sessions.sentID != "ao-1" || sessions.sentMessage != "report context" || sessions.sentKey != "report-batch:abc123" || sessions.sentAttempt != 1 {
		t.Fatalf("semantic send = %+v", sessions)
	}
	if err := delivery.Interrupt(context.Background(), "ao-1"); err != nil {
		t.Fatal(err)
	}
	if sessions.interrupted != "ao-1" {
		t.Fatalf("interrupted = %q", sessions.interrupted)
	}
}
