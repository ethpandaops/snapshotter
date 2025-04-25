package db

import (
	"database/sql"
	"fmt"

	log "github.com/sirupsen/logrus"
)

// Migration represents a database migration
type Migration struct {
	ID      int
	Name    string
	Migrate func(*sql.DB) error
}

// migrations is a list of all migrations to run, in order
var migrations = []Migration{
	{
		ID:      1,
		Name:    "Add deleted column to snapshot_runs and target_snapshots tables",
		Migrate: migrateAddDeletedColumn,
	},
}

// migrateAddDeletedColumn adds the deleted column to the snapshot_runs and target_snapshots tables
func migrateAddDeletedColumn(db *sql.DB) error {
	// Check if deleted column exists in snapshot_runs
	var snapshotRunsColumnExists int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('snapshot_runs')
		WHERE name='deleted'
	`).Scan(&snapshotRunsColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check if deleted column exists in snapshot_runs: %w", err)
	}

	// Add deleted column to snapshot_runs table if it doesn't exist
	if snapshotRunsColumnExists == 0 {
		_, err := db.Exec(`
			ALTER TABLE snapshot_runs
			ADD COLUMN deleted BOOLEAN NOT NULL DEFAULT 0
		`)
		if err != nil {
			return fmt.Errorf("failed to add deleted column to snapshot_runs: %w", err)
		}
	}

	// Check if deleted column exists in target_snapshots
	var targetSnapshotsColumnExists int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('target_snapshots')
		WHERE name='deleted'
	`).Scan(&targetSnapshotsColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check if deleted column exists in target_snapshots: %w", err)
	}

	// Add deleted column to target_snapshots table if it doesn't exist
	if targetSnapshotsColumnExists == 0 {
		_, err := db.Exec(`
			ALTER TABLE target_snapshots
			ADD COLUMN deleted BOOLEAN NOT NULL DEFAULT 0
		`)
		if err != nil {
			return fmt.Errorf("failed to add deleted column to target_snapshots: %w", err)
		}
	}

	return nil
}

// RunMigrations runs all database migrations
func RunMigrations(db *sql.DB) error {
	// Create migrations table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS migrations (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get the last applied migration
	var lastMigrationID int
	err = db.QueryRow(`
		SELECT COALESCE(MAX(id), 0) FROM migrations
	`).Scan(&lastMigrationID)
	if err != nil {
		return fmt.Errorf("failed to get last migration: %w", err)
	}

	// Apply any pending migrations
	for _, migration := range migrations {
		if migration.ID <= lastMigrationID {
			// Skip already applied migrations
			continue
		}

		log.WithFields(log.Fields{
			"id":   migration.ID,
			"name": migration.Name,
		}).Info("applying database migration")

		// Start a transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to start transaction: %w", err)
		}

		// Apply the migration
		if err := migration.Migrate(db); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to apply migration %d (%s): %w", migration.ID, migration.Name, err)
		}

		// Record the migration
		_, err = tx.Exec(`
			INSERT INTO migrations (id, name) VALUES (?, ?)
		`, migration.ID, migration.Name)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration: %w", err)
		}

		// Commit the transaction
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit transaction: %w", err)
		}

		log.WithFields(log.Fields{
			"id":   migration.ID,
			"name": migration.Name,
		}).Info("Successfully applied database migration")
	}

	return nil
}
