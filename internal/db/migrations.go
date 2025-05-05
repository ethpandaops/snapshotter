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
	{
		ID:      2,
		Name:    "Add persisted column to snapshot_runs table",
		Migrate: migrateAddPersistedColumn,
	},
	{
		ID:      3,
		Name:    "Add persisted column to target_snapshots table",
		Migrate: migrateAddPersistedColumnToTargetSnapshots,
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

// migrateAddPersistedColumn adds the persisted column to the snapshot_runs table
func migrateAddPersistedColumn(db *sql.DB) error {
	// Check if persisted column exists in snapshot_runs
	var snapshotRunsColumnExists int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('snapshot_runs')
		WHERE name='persisted'
	`).Scan(&snapshotRunsColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check if persisted column exists in snapshot_runs: %w", err)
	}

	// Add persisted column to snapshot_runs table if it doesn't exist
	if snapshotRunsColumnExists == 0 {
		_, err := db.Exec(`
			ALTER TABLE snapshot_runs
			ADD COLUMN persisted BOOLEAN NOT NULL DEFAULT 0
		`)
		if err != nil {
			return fmt.Errorf("failed to add persisted column to snapshot_runs: %w", err)
		}
	}

	return nil
}

// migrateAddPersistedColumnToTargetSnapshots adds the persisted column to the target_snapshots table
func migrateAddPersistedColumnToTargetSnapshots(db *sql.DB) error {
	// Check if persisted column exists in target_snapshots
	var targetSnapshotsColumnExists int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('target_snapshots')
		WHERE name='persisted'
	`).Scan(&targetSnapshotsColumnExists)
	if err != nil {
		return fmt.Errorf("failed to check if persisted column exists in target_snapshots: %w", err)
	}

	// Add persisted column to target_snapshots table if it doesn't exist
	if targetSnapshotsColumnExists == 0 {
		_, err := db.Exec(`
			ALTER TABLE target_snapshots
			ADD COLUMN persisted BOOLEAN NOT NULL DEFAULT 0
		`)
		if err != nil {
			return fmt.Errorf("failed to add persisted column to target_snapshots: %w", err)
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

	// Check which migrations have already been applied
	rows, err := db.Query("SELECT id FROM migrations ORDER BY id")
	if err != nil {
		return fmt.Errorf("failed to query migrations: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			log.WithError(cerr).Error("Failed to close rows")
		}
	}()

	appliedMigrations := make(map[int]bool)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan migration ID: %w", err)
		}
		appliedMigrations[id] = true
	}

	// Apply migrations that haven't been applied yet
	for _, migration := range migrations {
		if !appliedMigrations[migration.ID] {
			log.WithFields(log.Fields{
				"id":   migration.ID,
				"name": migration.Name,
			}).Info("Applying migration")

			// Run the migration in a transaction
			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}

			if err := migration.Migrate(db); err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					log.WithError(rbErr).Error("Failed to rollback transaction")
				}
				return fmt.Errorf("failed to apply migration %d: %w", migration.ID, err)
			}

			// Record the migration as applied
			_, err = tx.Exec("INSERT INTO migrations (id, name) VALUES (?, ?)", migration.ID, migration.Name)
			if err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					log.WithError(rbErr).Error("Failed to rollback transaction")
				}
				return fmt.Errorf("failed to record migration %d: %w", migration.ID, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration %d: %w", migration.ID, err)
			}

			log.WithFields(log.Fields{
				"id":   migration.ID,
				"name": migration.Name,
			}).Info("Successfully applied migration")
		}
	}

	return nil
}
