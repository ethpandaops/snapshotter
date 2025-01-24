package db

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	db *sql.DB
}

type SnapshotRun struct {
	ID              int64
	BlockHeight     uint64
	StartTime       time.Time
	EndTime         time.Time
	Status          string // "success" or "failed"
	ErrorMessage    string
	DryRun          bool
	TargetsSnapshot []TargetSnapshot
}

type TargetSnapshot struct {
	ID            int64
	SnapshotRunID int64
	Alias         string
	UploadPrefix  string
	StartTime     time.Time
	EndTime       time.Time
	Status        string // "success" or "failed"
	ErrorMessage  string
	DryRun        bool
}

func NewDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := initSchema(db); err != nil {
		return nil, err
	}

	return &DB{db: db}, nil
}

func initSchema(db *sql.DB) error {
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

	_, err := db.Exec(schema)
	return err
}

func (d *DB) CreateSnapshotRun(blockHeight uint64, dryRun bool) (*SnapshotRun, error) {
	result, err := d.db.Exec(
		"INSERT INTO snapshot_runs (block_height, start_time, status, dry_run) VALUES (?, ?, ?, ?)",
		blockHeight,
		time.Now(),
		"running",
		dryRun,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &SnapshotRun{
		ID:          id,
		BlockHeight: blockHeight,
		StartTime:   time.Now(),
		Status:      "running",
		DryRun:      dryRun,
	}, nil
}

func (d *DB) UpdateSnapshotRunStatus(id int64, status string, errorMsg string) error {
	_, err := d.db.Exec(
		"UPDATE snapshot_runs SET status = ?, error_message = ?, end_time = ? WHERE id = ?",
		status,
		errorMsg,
		time.Now(),
		id,
	)
	return err
}

func (d *DB) CreateTargetSnapshot(runID int64, alias, uploadPrefix string, dryRun bool) (*TargetSnapshot, error) {
	result, err := d.db.Exec(
		"INSERT INTO target_snapshots (snapshot_run_id, alias, upload_prefix, start_time, status, dry_run) VALUES (?, ?, ?, ?, ?, ?)",
		runID,
		alias,
		uploadPrefix,
		time.Now(),
		"running",
		dryRun,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &TargetSnapshot{
		ID:            id,
		SnapshotRunID: runID,
		Alias:         alias,
		UploadPrefix:  uploadPrefix,
		StartTime:     time.Now(),
		Status:        "running",
		DryRun:        dryRun,
	}, nil
}

func (d *DB) UpdateTargetSnapshotStatus(id int64, status string, errorMsg string) error {
	_, err := d.db.Exec(
		"UPDATE target_snapshots SET status = ?, error_message = ?, end_time = ? WHERE id = ?",
		status,
		errorMsg,
		time.Now(),
		id,
	)
	return err
}
