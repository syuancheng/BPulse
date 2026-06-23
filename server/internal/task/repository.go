package task

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/syuancheng/BPulse/server/internal/careplan"
)

type Repository interface {
	CreateForPlans(ctx context.Context, ownerID uint64, localDate string, location *time.Location, plans []careplan.Plan) error
	ListForLocalDate(ctx context.Context, ownerID uint64, localDate string) ([]Instance, error)
	SetState(ctx context.Context, ownerID, taskID uint64, state string, nowUTC time.Time) (Instance, error)
}

type MySQLRepository struct {
	db           *sql.DB
	queryTimeout time.Duration
}

func NewMySQLRepository(db *sql.DB, queryTimeout time.Duration) *MySQLRepository {
	return &MySQLRepository{db: db, queryTimeout: queryTimeout}
}

func (r *MySQLRepository) CreateForPlans(ctx context.Context, ownerID uint64, localDate string, location *time.Location, plans []careplan.Plan) error {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	for _, plan := range plans {
		scheduledAtUTC, err := scheduledUTC(localDate, plan.LocalTime, location)
		if err != nil {
			return err
		}
		if _, err := r.db.ExecContext(queryCtx, `
INSERT IGNORE INTO task_instances (owner_id, care_plan_id, task_type, title, scheduled_local_date, occurrence_key, scheduled_at_utc)
VALUES (?, ?, ?, ?, ?, 'daily', ?)`, ownerID, plan.ID, plan.TaskType, plan.Title, localDate, scheduledAtUTC); err != nil {
			return fmt.Errorf("create task instance: %w", err)
		}
	}
	return nil
}

func (r *MySQLRepository) ListForLocalDate(ctx context.Context, ownerID uint64, localDate string) ([]Instance, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(queryCtx, `
SELECT id, owner_id, care_plan_id, task_type, title, scheduled_local_date, occurrence_key, scheduled_at_utc, state, completed_at_utc, skipped_at_utc, created_at, updated_at
FROM task_instances
WHERE owner_id = ? AND scheduled_local_date = ?
ORDER BY scheduled_at_utc, id`, ownerID, localDate)
	if err != nil {
		return nil, fmt.Errorf("list task instances: %w", err)
	}
	defer rows.Close()
	return scanInstances(rows)
}

func (r *MySQLRepository) SetState(ctx context.Context, ownerID, taskID uint64, state string, nowUTC time.Time) (Instance, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	completedAt := sql.NullTime{}
	skippedAt := sql.NullTime{}
	if state == StateCompleted {
		completedAt = sql.NullTime{Time: nowUTC.UTC(), Valid: true}
	} else if state == StateSkipped {
		skippedAt = sql.NullTime{Time: nowUTC.UTC(), Valid: true}
	}
	result, err := r.db.ExecContext(queryCtx, `
UPDATE task_instances
SET completed_at_utc = CASE WHEN state = 'pending' AND ? = 'completed' THEN ? ELSE completed_at_utc END,
    skipped_at_utc = CASE WHEN state = 'pending' AND ? = 'skipped' THEN ? ELSE skipped_at_utc END,
    state = CASE WHEN state = 'pending' THEN ? ELSE state END
WHERE id = ? AND owner_id = ?`, state, completedAt, state, skippedAt, state, taskID, ownerID)
	if err != nil {
		return Instance{}, fmt.Errorf("update task state: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Instance{}, fmt.Errorf("read task update result: %w", err)
	}
	if affected == 0 {
		return r.get(queryCtx, ownerID, taskID)
	}
	return r.get(queryCtx, ownerID, taskID)
}

func (r *MySQLRepository) get(ctx context.Context, ownerID, taskID uint64) (Instance, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, owner_id, care_plan_id, task_type, title, scheduled_local_date, occurrence_key, scheduled_at_utc, state, completed_at_utc, skipped_at_utc, created_at, updated_at
FROM task_instances
WHERE id = ? AND owner_id = ?`, taskID, ownerID)
	if err != nil {
		return Instance{}, fmt.Errorf("get task instance: %w", err)
	}
	defer rows.Close()
	instances, err := scanInstances(rows)
	if err != nil {
		return Instance{}, err
	}
	if len(instances) == 0 {
		return Instance{}, ErrNotFound
	}
	return instances[0], nil
}

func (r *MySQLRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := r.queryTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func scheduledUTC(localDate, localTime string, location *time.Location) (time.Time, error) {
	parsed, err := time.ParseInLocation("2006-01-02 15:04", localDate+" "+localTime, location)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse scheduled local time: %w", err)
	}
	return parsed.UTC(), nil
}

func scanInstances(rows *sql.Rows) ([]Instance, error) {
	instances := []Instance{}
	for rows.Next() {
		var instance Instance
		var localDate time.Time
		var completedAt, skippedAt sql.NullTime
		if err := rows.Scan(&instance.ID, &instance.OwnerID, &instance.CarePlanID, &instance.TaskType, &instance.Title, &localDate, &instance.OccurrenceKey, &instance.ScheduledAtUTC, &instance.State, &completedAt, &skippedAt, &instance.CreatedAt, &instance.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task instance: %w", err)
		}
		instance.ScheduledLocalDate = localDate.Format("2006-01-02")
		instance.ScheduledAtUTC = instance.ScheduledAtUTC.UTC()
		instance.CreatedAt = instance.CreatedAt.UTC()
		instance.UpdatedAt = instance.UpdatedAt.UTC()
		if completedAt.Valid {
			value := completedAt.Time.UTC()
			instance.CompletedAtUTC = &value
		}
		if skippedAt.Valid {
			value := skippedAt.Time.UTC()
			instance.SkippedAtUTC = &value
		}
		instances = append(instances, instance)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task instances: %w", err)
	}
	return instances, nil
}
