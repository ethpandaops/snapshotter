package db

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrations(t *testing.T) {
	// Create a temporary database file
	dbPath := "test_migrations.db"
	defer os.Remove(dbPath)

	// Open the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize the schema (without deleted columns)
	schema := `
	CREATE TABLE IF NOT EXISTS snapshot_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		block_height INTEGER NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		status TEXT NOT NULL,
		error_message TEXT,
		dry_run BOOLEAN NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS target_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		snapshot_run_id INTEGER NOT NULL,
		alias TEXT NOT NULL,
		upload_prefix TEXT NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		status TEXT NOT NULL,
		error_message TEXT,
		dry_run BOOLEAN NOT NULL DEFAULT 0,
		FOREIGN KEY(snapshot_run_id) REFERENCES snapshot_runs(id)
	);`

	_, err = db.Exec(schema)
	if err != nil {
		t.Fatalf("Failed to initialize schema: %v", err)
	}

	// Run migrations
	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Check if the migrations table was created
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query migrations table: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 migration record, got %d", count)
	}

	// Check if the deleted columns were added
	var columnCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('snapshot_runs')
		WHERE name='deleted'
	`).Scan(&columnCount)
	if err != nil {
		t.Fatalf("Failed to check snapshot_runs table: %v", err)
	}
	if columnCount != 1 {
		t.Errorf("Expected deleted column in snapshot_runs, but it wasn't found")
	}

	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('target_snapshots')
		WHERE name='deleted'
	`).Scan(&columnCount)
	if err != nil {
		t.Fatalf("Failed to check target_snapshots table: %v", err)
	}
	if columnCount != 1 {
		t.Errorf("Expected deleted column in target_snapshots, but it wasn't found")
	}

	// Check if a second run of migrations doesn't duplicate the migration
	err = RunMigrations(db)
	if err != nil {
		t.Fatalf("Failed to run migrations second time: %v", err)
	}

	err = db.QueryRow("SELECT COUNT(*) FROM migrations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query migrations table: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected still 1 migration record after second run, got %d", count)
	}
}
