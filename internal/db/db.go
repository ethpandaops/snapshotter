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
	ID              int64     `json:"id"`
	BlockHeight     uint64    `json:"blockHeight"`
	StartTime       time.Time `json:"startTime"`
	EndTime         time.Time `json:"endTime"`
	Status          string    `json:"status"` // "success" or "failed"
	ErrorMessage    string    `json:"errorMessage"`
	DryRun          bool      `json:"dryRun"`
	TargetsSnapshot []TargetSnapshot
}

type TargetSnapshot struct {
	ID            int64     `json:"id"`
	SnapshotRunID int64     `json:"snapshotRunId"`
	Alias         string    `json:"alias"`
	UploadPrefix  string    `json:"uploadPrefix"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime"`
	Status        string    `json:"status"` // "success" or "failed"
	ErrorMessage  string    `json:"errorMessage"`
	DryRun        bool      `json:"isDryRun"`
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

func (d *DB) GetTargetSnapshotsForRun(runID int64) (targets []TargetSnapshot, err error) {
	rows, err := d.db.Query(`
		SELECT id, snapshot_run_id, alias, upload_prefix, start_time, end_time, status, error_message, dry_run
		FROM target_snapshots
		WHERE snapshot_run_id = ?
		ORDER BY start_time ASC
	`, runID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			if err == nil {
				err = cerr
			}
		}
	}()

	targets = []TargetSnapshot{}
	for rows.Next() {
		var target TargetSnapshot
		var endTime sql.NullTime
		var errorMessage sql.NullString
		err := rows.Scan(
			&target.ID,
			&target.SnapshotRunID,
			&target.Alias,
			&target.UploadPrefix,
			&target.StartTime,
			&endTime,
			&target.Status,
			&errorMessage,
			&target.DryRun,
		)
		if err != nil {
			return nil, err
		}
		if endTime.Valid {
			target.EndTime = endTime.Time
		}
		if errorMessage.Valid {
			target.ErrorMessage = errorMessage.String
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func (d *DB) GetAllRuns() (runs []SnapshotRun, err error) {
	rows, err := d.db.Query(`
		SELECT id, block_height, start_time, end_time, status, error_message, dry_run
		FROM snapshot_runs
		ORDER BY start_time DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			if err == nil {
				err = cerr
			}
		}
	}()

	runs = []SnapshotRun{}
	for rows.Next() {
		var run SnapshotRun
		var endTime sql.NullTime
		var errorMessage sql.NullString
		err := rows.Scan(
			&run.ID,
			&run.BlockHeight,
			&run.StartTime,
			&endTime,
			&run.Status,
			&errorMessage,
			&run.DryRun,
		)
		if err != nil {
			return nil, err
		}
		if endTime.Valid {
			run.EndTime = endTime.Time
		}
		if errorMessage.Valid {
			run.ErrorMessage = errorMessage.String
		}

		// Get associated target snapshots
		targets, err := d.GetTargetSnapshotsForRun(run.ID)
		if err != nil {
			return nil, err
		}
		run.TargetsSnapshot = targets
		runs = append(runs, run)
	}
	return runs, nil
}

func (d *DB) GetMostRecentRun() (*SnapshotRun, error) {
	row := d.db.QueryRow(`
		SELECT id, block_height, start_time, end_time, status, error_message, dry_run
		FROM snapshot_runs
		ORDER BY start_time DESC
		LIMIT 1
	`)

	var run SnapshotRun
	var endTime sql.NullTime
	var errorMessage sql.NullString
	err := row.Scan(
		&run.ID,
		&run.BlockHeight,
		&run.StartTime,
		&endTime,
		&run.Status,
		&errorMessage,
		&run.DryRun,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if endTime.Valid {
		run.EndTime = endTime.Time
	}
	if errorMessage.Valid {
		run.ErrorMessage = errorMessage.String
	}

	targets, err := d.GetTargetSnapshotsForRun(run.ID)
	if err != nil {
		return nil, err
	}
	run.TargetsSnapshot = targets

	return &run, nil
}

func (d *DB) GetPaginatedRuns(offset, limit int) (runs []SnapshotRun, err error) {
	if limit > 20 {
		limit = 20
	}

	rows, err := d.db.Query(`
		SELECT id, block_height, start_time, end_time, status, error_message, dry_run
		FROM snapshot_runs
		ORDER BY start_time DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			if err == nil {
				err = cerr
			}
		}
	}()

	runs = []SnapshotRun{}
	for rows.Next() {
		var run SnapshotRun
		var endTime sql.NullTime
		var errorMessage sql.NullString
		err := rows.Scan(
			&run.ID,
			&run.BlockHeight,
			&run.StartTime,
			&endTime,
			&run.Status,
			&errorMessage,
			&run.DryRun,
		)
		if err != nil {
			return nil, err
		}
		if endTime.Valid {
			run.EndTime = endTime.Time
		}
		if errorMessage.Valid {
			run.ErrorMessage = errorMessage.String
		}

		targets, err := d.GetTargetSnapshotsForRun(run.ID)
		if err != nil {
			return nil, err
		}
		run.TargetsSnapshot = targets
		runs = append(runs, run)
	}
	return runs, nil
}
