package models

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type DiscoveryRunRepository struct {
	conn   queryConn
	rootDB *sql.DB
}

func NewDiscoveryRunRepository(database *sql.DB) *DiscoveryRunRepository {
	return &DiscoveryRunRepository{conn: database, rootDB: database}
}

func (r *DiscoveryRunRepository) Create(ctx context.Context, run *DiscoveryRun) error {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if run.Status == "" {
		run.Status = "queued"
	}

	now := nowUTC()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = run.CreatedAt
	}

	_, err := r.conn.ExecContext(ctx, `
		INSERT INTO discovery_runs (
			id, profile, target, status, started_at, completed_at, current_phase,
			progress_percent, hosts_found, services_found, observations_count, error,
			logs, raw_output_path, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		run.ID,
		run.Profile,
		run.Target,
		run.Status,
		formatNullableTime(run.StartedAt),
		formatNullableTime(run.CompletedAt),
		run.CurrentPhase,
		run.ProgressPercent,
		run.HostsFound,
		run.ServicesFound,
		run.ObservationsCount,
		run.Error,
		run.Logs,
		run.RawOutputPath,
		formatRequiredTime(run.CreatedAt),
		formatRequiredTime(run.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("create discovery run: %w", err)
	}
	return nil
}

func (r *DiscoveryRunRepository) Update(ctx context.Context, run *DiscoveryRun) error {
	run.UpdatedAt = nowUTC()
	_, err := r.conn.ExecContext(ctx, `
		UPDATE discovery_runs
		SET status = ?, started_at = ?, completed_at = ?, current_phase = ?,
			progress_percent = ?, hosts_found = ?, services_found = ?, observations_count = ?,
			error = ?, logs = ?, raw_output_path = ?, updated_at = ?
		WHERE id = ?
	`,
		run.Status,
		formatNullableTime(run.StartedAt),
		formatNullableTime(run.CompletedAt),
		run.CurrentPhase,
		run.ProgressPercent,
		run.HostsFound,
		run.ServicesFound,
		run.ObservationsCount,
		run.Error,
		run.Logs,
		run.RawOutputPath,
		formatRequiredTime(run.UpdatedAt),
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("update discovery run: %w", err)
	}
	return nil
}

func (r *DiscoveryRunRepository) SetCancelled(ctx context.Context, id string) error {
	now := formatRequiredTime(nowUTC())
	_, err := r.conn.ExecContext(ctx,
		`UPDATE discovery_runs SET status='cancelled', completed_at=?, updated_at=? WHERE id=? AND status IN ('queued','running')`,
		now, now, id,
	)
	return err
}

func (r *DiscoveryRunRepository) Latest(ctx context.Context) (*DiscoveryRun, error) {
	row := r.conn.QueryRowContext(ctx, `
		SELECT
			id, profile, target, status, started_at, completed_at, current_phase,
			progress_percent, hosts_found, services_found, observations_count,
			error, logs, raw_output_path, created_at, updated_at
		FROM discovery_runs
		ORDER BY created_at DESC
		LIMIT 1
	`)

	run, err := scanDiscoveryRun(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &run, nil
}

func (r *DiscoveryRunRepository) ListRecent(ctx context.Context, limit int) ([]DiscoveryRun, error) {
	rows, err := r.conn.QueryContext(ctx, `
		SELECT
			id, profile, target, status, started_at, completed_at, current_phase,
			progress_percent, hosts_found, services_found, observations_count,
			error, logs, raw_output_path, created_at, updated_at
		FROM discovery_runs
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list discovery runs: %w", err)
	}
	defer rows.Close()

	var runs []DiscoveryRun
	for rows.Next() {
		run, err := scanDiscoveryRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate discovery runs: %w", err)
	}
	return runs, nil
}

func scanDiscoveryRun(scanner assetScanner) (DiscoveryRun, error) {
	var run DiscoveryRun
	var currentPhase sql.NullString
	var runError sql.NullString
	var rawOutputPath sql.NullString
	var startedAt sql.NullString
	var completedAt sql.NullString
	var createdAt sql.NullString
	var updatedAt sql.NullString

	err := scanner.Scan(
		&run.ID,
		&run.Profile,
		&run.Target,
		&run.Status,
		&startedAt,
		&completedAt,
		&currentPhase,
		&run.ProgressPercent,
		&run.HostsFound,
		&run.ServicesFound,
		&run.ObservationsCount,
		&runError,
		&run.Logs,
		&rawOutputPath,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return DiscoveryRun{}, err
	}

	run.CurrentPhase = currentPhase.String
	run.Error = runError.String
	run.RawOutputPath = rawOutputPath.String

	run.StartedAt, err = parseNullableTime(startedAt)
	if err != nil {
		return DiscoveryRun{}, err
	}
	run.CompletedAt, err = parseNullableTime(completedAt)
	if err != nil {
		return DiscoveryRun{}, err
	}
	run.CreatedAt, err = parseNullableTime(createdAt)
	if err != nil {
		return DiscoveryRun{}, err
	}
	run.UpdatedAt, err = parseNullableTime(updatedAt)
	if err != nil {
		return DiscoveryRun{}, err
	}

	return run, nil
}
