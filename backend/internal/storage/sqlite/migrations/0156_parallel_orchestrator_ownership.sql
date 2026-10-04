-- +goose Up
ALTER TABLE sessions ADD COLUMN orchestrator_session_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN parallel_orchestrator BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX idx_sessions_orchestrator_owner ON sessions(orchestrator_session_id)
    WHERE orchestrator_session_id <> '';
ALTER TABLE reports ADD COLUMN orchestrator_session_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_reports_orchestrator_delivery
    ON reports(project_id, orchestrator_session_id, delivery_state, available_at);

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_orchestrator_owner;
DROP INDEX IF EXISTS idx_reports_orchestrator_delivery;
ALTER TABLE reports DROP COLUMN orchestrator_session_id;
ALTER TABLE sessions DROP COLUMN parallel_orchestrator;
ALTER TABLE sessions DROP COLUMN orchestrator_session_id;
