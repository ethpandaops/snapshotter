package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	db *sql.DB
}

type SnapshotRun struct {
	ID              int64            `json:"id"`
	BlockHeight     uint64           `json:"blockHeight"`
	StartTime       time.Time        `json:"startTime"`
	EndTime         time.Time        `json:"endTime"`
	Status          string           `json:"status"` // "success" or "failed"
	ErrorMessage    string           `json:"errorMessage"`
	DryRun          bool             `json:"dryRun"`
	Deleted         bool             `json:"deleted"`
	Persisted       bool             `json:"persisted"`
	TargetsSnapshot []TargetSnapshot `json:"targets"`
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
	Deleted       bool      `json:"deleted"`
	Persisted     bool      `json:"persisted"`
}

func NewDB(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Initialize schema first
	if err := initSchema(db); err != nil {
		return nil, err
	}

	// Run migrations after schema initialization
	if err := RunMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
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
		SELECT id, snapshot_run_id, alias, upload_prefix, start_time, end_time, status, error_message, dry_run, deleted, persisted
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
		var persisted sql.NullBool
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
			&target.Deleted,
			&persisted,
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
		if persisted.Valid {
			target.Persisted = persisted.Bool
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func (d *DB) GetAllRuns() (runs []SnapshotRun, err error) {
	rows, err := d.db.Query(`
		SELECT id, block_height, start_time, end_time, status, error_message, dry_run, deleted, persisted
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
		var persisted sql.NullBool
		err := rows.Scan(
			&run.ID,
			&run.BlockHeight,
			&run.StartTime,
			&endTime,
			&run.Status,
			&errorMessage,
			&run.DryRun,
			&run.Deleted,
			&persisted,
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
		if persisted.Valid {
			run.Persisted = persisted.Bool
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
		SELECT id, block_height, start_time, end_time, status, error_message, dry_run, deleted, persisted
		FROM snapshot_runs
		ORDER BY start_time DESC
		LIMIT 1
	`)

	var run SnapshotRun
	var endTime sql.NullTime
	var errorMessage sql.NullString
	var persisted sql.NullBool
	err := row.Scan(
		&run.ID,
		&run.BlockHeight,
		&run.StartTime,
		&endTime,
		&run.Status,
		&errorMessage,
		&run.DryRun,
		&run.Deleted,
		&persisted,
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
	if persisted.Valid {
		run.Persisted = persisted.Bool
	}

	targets, err := d.GetTargetSnapshotsForRun(run.ID)
	if err != nil {
		return nil, err
	}
	run.TargetsSnapshot = targets

	return &run, nil
}

func (d *DB) GetPaginatedRuns(offset, limit int, includeDeleted bool, onlyPersisted bool) (runs []SnapshotRun, err error) {
	if limit > 20 {
		limit = 20
	}

	var query string
	var args []interface{}

	// Build query based on filter options
	switch {
	case !includeDeleted && onlyPersisted:
		query = `
			SELECT id, block_height, start_time, end_time, status, error_message, dry_run, deleted, persisted
			FROM snapshot_runs
			WHERE deleted = 0 AND persisted = 1
			ORDER BY start_time DESC
			LIMIT ? OFFSET ?
		`
	case includeDeleted && onlyPersisted:
		query = `
			SELECT id, block_height, start_time, end_time, status, error_message, dry_run, deleted, persisted
			FROM snapshot_runs
			WHERE persisted = 1
			ORDER BY start_time DESC
			LIMIT ? OFFSET ?
		`
	case includeDeleted && !onlyPersisted:
		query = `
			SELECT id, block_height, start_time, end_time, status, error_message, dry_run, deleted, persisted
			FROM snapshot_runs
			ORDER BY start_time DESC
			LIMIT ? OFFSET ?
		`
	default: // !includeDeleted && !onlyPersisted
		query = `
			SELECT id, block_height, start_time, end_time, status, error_message, dry_run, deleted, persisted
			FROM snapshot_runs
			WHERE deleted = 0
			ORDER BY start_time DESC
			LIMIT ? OFFSET ?
		`
	}

	args = []interface{}{limit, offset}
	rows, err := d.db.Query(query, args...)
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
		var persisted sql.NullBool
		err := rows.Scan(
			&run.ID,
			&run.BlockHeight,
			&run.StartTime,
			&endTime,
			&run.Status,
			&errorMessage,
			&run.DryRun,
			&run.Deleted,
			&persisted,
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
		if persisted.Valid {
			run.Persisted = persisted.Bool
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

func (d *DB) GetSuccessfulRunsForCleanup() (runs []SnapshotRun, err error) {
	rows, err := d.db.Query(`
		SELECT id, block_height, start_time, end_time, status, error_message, dry_run, deleted, persisted
		FROM snapshot_runs
		WHERE status = 'success' AND deleted = 0
		ORDER BY block_height DESC
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
		var deleted bool
		var persisted bool
		err := rows.Scan(
			&run.ID,
			&run.BlockHeight,
			&run.StartTime,
			&endTime,
			&run.Status,
			&errorMessage,
			&run.DryRun,
			&deleted,
			&persisted,
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
		run.Persisted = persisted

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

func (d *DB) MarkSnapshotRunAsDeleted(id int64) error {
	_, err := d.db.Exec(
		"UPDATE snapshot_runs SET deleted = 1 WHERE id = ?",
		id,
	)
	if err != nil {
		return err
	}

	// Also mark all associated target snapshots as deleted
	_, err = d.db.Exec(
		"UPDATE target_snapshots SET deleted = 1 WHERE snapshot_run_id = ?",
		id,
	)
	return err
}

// Set a snapshot run as persisted or not persisted
func (d *DB) SetSnapshotRunPersisted(id int64, persisted bool) error {
	_, err := d.db.Exec(
		"UPDATE snapshot_runs SET persisted = ? WHERE id = ?",
		persisted,
		id,
	)
	if err != nil {
		return err
	}

	// Also update all associated target snapshots
	_, err = d.db.Exec(
		"UPDATE target_snapshots SET persisted = ? WHERE snapshot_run_id = ?",
		persisted,
		id,
	)
	return err
}

// Get a single snapshot run by ID
func (d *DB) GetSnapshotRunByID(id int64) (*SnapshotRun, error) {
	row := d.db.QueryRow(`
		SELECT id, block_height, start_time, end_time, status, error_message, dry_run, deleted, persisted
		FROM snapshot_runs
		WHERE id = ?
	`, id)

	var run SnapshotRun
	var endTime sql.NullTime
	var errorMessage sql.NullString
	var persisted sql.NullBool
	err := row.Scan(
		&run.ID,
		&run.BlockHeight,
		&run.StartTime,
		&endTime,
		&run.Status,
		&errorMessage,
		&run.DryRun,
		&run.Deleted,
		&persisted,
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
	if persisted.Valid {
		run.Persisted = persisted.Bool
	}

	targets, err := d.GetTargetSnapshotsForRun(run.ID)
	if err != nil {
		return nil, err
	}
	run.TargetsSnapshot = targets

	return &run, nil
}

// Set a target snapshot as persisted or not persisted
func (d *DB) SetTargetSnapshotPersisted(id int64, persisted bool) error {
	_, err := d.db.Exec(
		"UPDATE target_snapshots SET persisted = ? WHERE id = ?",
		persisted,
		id,
	)
	return err
}

// Get a single target snapshot by ID
func (d *DB) GetTargetSnapshotByID(id int64) (*TargetSnapshot, error) {
	row := d.db.QueryRow(`
		SELECT id, snapshot_run_id, alias, upload_prefix, start_time, end_time, status, error_message, dry_run, deleted, persisted
		FROM target_snapshots
		WHERE id = ?
	`, id)

	var target TargetSnapshot
	var endTime sql.NullTime
	var errorMessage sql.NullString
	var persisted sql.NullBool
	err := row.Scan(
		&target.ID,
		&target.SnapshotRunID,
		&target.Alias,
		&target.UploadPrefix,
		&target.StartTime,
		&endTime,
		&target.Status,
		&errorMessage,
		&target.DryRun,
		&target.Deleted,
		&persisted,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if endTime.Valid {
		target.EndTime = endTime.Time
	}
	if errorMessage.Valid {
		target.ErrorMessage = errorMessage.String
	}
	if persisted.Valid {
		target.Persisted = persisted.Bool
	}

	return &target, nil
}

// Get all target snapshots for cleanup that are successful and not deleted
func (d *DB) GetSuccessfulTargetSnapshotsForCleanup() (targets []TargetSnapshot, err error) {
	rows, err := d.db.Query(`
		SELECT id, snapshot_run_id, alias, upload_prefix, start_time, end_time, status, error_message, dry_run, deleted, persisted
		FROM target_snapshots
		WHERE status = 'success' AND deleted = 0
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

	targets = []TargetSnapshot{}
	for rows.Next() {
		var target TargetSnapshot
		var endTime sql.NullTime
		var errorMessage sql.NullString
		var persisted sql.NullBool
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
			&target.Deleted,
			&persisted,
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
		if persisted.Valid {
			target.Persisted = persisted.Bool
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// Mark a single target snapshot as deleted
func (d *DB) MarkTargetSnapshotAsDeleted(id int64) error {
	_, err := d.db.Exec(
		"UPDATE target_snapshots SET deleted = 1 WHERE id = ?",
		id,
	)
	return err
}

// GetTargetSnapshotsByAlias gets all target snapshots matching the given alias
func (d *DB) GetTargetSnapshotsByAlias(alias string, limit, offset int, includeDeleted bool, onlyPersisted bool) (targets []TargetSnapshot, err error) {
	if limit > 20 {
		limit = 20
	}

	var query string
	var args []interface{}

	// Build query based on filter options
	switch {
	case !includeDeleted && onlyPersisted:
		query = `
			SELECT id, snapshot_run_id, alias, upload_prefix, start_time, end_time, status, error_message, dry_run, deleted, persisted
			FROM target_snapshots
			WHERE alias = ? AND deleted = 0 AND persisted = 1
			ORDER BY start_time DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{alias, limit, offset}
	case includeDeleted && onlyPersisted:
		query = `
			SELECT id, snapshot_run_id, alias, upload_prefix, start_time, end_time, status, error_message, dry_run, deleted, persisted
			FROM target_snapshots
			WHERE alias = ? AND persisted = 1
			ORDER BY start_time DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{alias, limit, offset}
	case includeDeleted && !onlyPersisted:
		query = `
			SELECT id, snapshot_run_id, alias, upload_prefix, start_time, end_time, status, error_message, dry_run, deleted, persisted
			FROM target_snapshots
			WHERE alias = ?
			ORDER BY start_time DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{alias, limit, offset}
	default: // !includeDeleted && !onlyPersisted
		query = `
			SELECT id, snapshot_run_id, alias, upload_prefix, start_time, end_time, status, error_message, dry_run, deleted, persisted
			FROM target_snapshots
			WHERE alias = ? AND deleted = 0
			ORDER BY start_time DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{alias, limit, offset}
	}

	rows, err := d.db.Query(query, args...)
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
		var persisted sql.NullBool
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
			&target.Deleted,
			&persisted,
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
		if persisted.Valid {
			target.Persisted = persisted.Bool
		}
		targets = append(targets, target)
	}
	return targets, nil
}
