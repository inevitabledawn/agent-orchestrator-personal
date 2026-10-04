package sqlite

import "database/sql"

// The personal fork used 0156 for ownership before upstream assigned it to
// provisioning. Identify the old physical schema before remapping history; never
// replay either migration over columns that already exist or drop ownership data.
func repairPersonalOrchestratorOwnershipHistory(db *sql.DB) error {
	var ownershipColumns, provisionColumns int
	if err := db.QueryRow(`SELECT
  (SELECT COUNT(*) FROM pragma_table_info('sessions') WHERE name IN ('orchestrator_session_id', 'parallel_orchestrator')) +
  (SELECT COUNT(*) FROM pragma_table_info('reports') WHERE name = 'orchestrator_session_id'),
  (SELECT COUNT(*) FROM pragma_table_info('sessions') WHERE name IN ('provision_state', 'provision_error'))`).Scan(&ownershipColumns, &provisionColumns); err != nil {
		return err
	}
	if ownershipColumns != 3 || provisionColumns != 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var legacyApplied int
	if err := tx.QueryRow(`SELECT COALESCE((SELECT is_applied FROM goose_db_version WHERE version_id = 156 ORDER BY id DESC LIMIT 1), 0)`).Scan(&legacyApplied); err != nil {
		return err
	}
	if legacyApplied != 1 {
		return nil
	}
	if _, err := tx.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (171, 1)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM goose_db_version WHERE version_id = 156`); err != nil {
		return err
	}
	return tx.Commit()
}
