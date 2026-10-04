package sqlite

import (
	"testing"
	"testing/fstest"

	"github.com/pressly/goose/v3"
)

func TestMigrateRepairsPersonalOrchestratorOwnershipHistory(t *testing.T) {
	db := openMigratedDatabaseCopy(t, 155)
	contents, err := migrationsFS.ReadFile("migrations/0171_parallel_orchestrator_ownership.sql")
	if err != nil {
		t.Fatal(err)
	}
	gooseMu.Lock()
	goose.SetBaseFS(fstest.MapFS{"migrations/0156_parallel_orchestrator_ownership.sql": &fstest.MapFile{Data: contents}})
	goose.SetLogger(goose.NopLogger())
	err = goose.SetDialect("sqlite3")
	if err == nil {
		err = goose.Up(db, "migrations")
	}
	gooseMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
 INSERT INTO projects (id, path, registered_at) VALUES ('personal', '/scratch/personal', CURRENT_TIMESTAMP);
 INSERT INTO sessions (id, project_id, num, kind, harness, activity_last_at, created_at, updated_at, orchestrator_session_id, parallel_orchestrator)
 VALUES ('owner', 'personal', 1, 'orchestrator', 'codex', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, '', 1),
 ('worker', 'personal', 2, 'worker', 'codex', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'owner', 0);
 INSERT INTO reports (id, session_id, project_id, created_at, available_at, orchestrator_session_id)
 VALUES ('report', 'worker', 'personal', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'owner');`)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := migrate(db); err != nil {
			t.Fatalf("upgrade personal database: %v", err)
		}
	}
	var owner, reportOwner, provision string
	var parallel bool
	if err := db.QueryRow(`SELECT orchestrator_session_id, provision_state FROM sessions WHERE id = 'worker'`).Scan(&owner, &provision); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT parallel_orchestrator FROM sessions WHERE id = 'owner'`).Scan(&parallel); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT orchestrator_session_id FROM reports WHERE id = 'report'`).Scan(&reportOwner); err != nil {
		t.Fatal(err)
	}
	if owner != "owner" || reportOwner != "owner" || !parallel || provision != "ready" {
		t.Fatalf("upgrade lost facts: owner=%q reportOwner=%q parallel=%v provision=%q", owner, reportOwner, parallel, provision)
	}
	for _, version := range []int{156, 171} {
		var applied int
		if err := db.QueryRow(`SELECT is_applied FROM goose_db_version WHERE version_id = ? ORDER BY id DESC LIMIT 1`, version).Scan(&applied); err != nil || applied != 1 {
			t.Fatalf("migration %d: applied=%d err=%v", version, applied, err)
		}
	}
}

func TestMigrateUpstreamDatabaseAddsOrchestratorOwnership(t *testing.T) {
	db := openMigratedDatabaseCopy(t, 170)
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	var columns int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('sessions') WHERE name IN ('orchestrator_session_id', 'parallel_orchestrator', 'provision_state', 'automation_run_id')`).Scan(&columns); err != nil || columns != 4 {
		t.Fatalf("merged schema columns=%d err=%v", columns, err)
	}
}
