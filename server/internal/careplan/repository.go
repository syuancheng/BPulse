package careplan

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository interface {
	Create(ctx context.Context, input CreatePlanInput) (Plan, error)
	List(ctx context.Context, ownerID uint64) ([]Plan, error)
	Update(ctx context.Context, input UpdatePlanInput) (Plan, error)
	Delete(ctx context.Context, ownerID, planID uint64) error
	ListEnabled(ctx context.Context, ownerID uint64) ([]Plan, error)
}

type MySQLRepository struct {
	db           *sql.DB
	queryTimeout time.Duration
}

func NewMySQLRepository(db *sql.DB, queryTimeout time.Duration) *MySQLRepository {
	return &MySQLRepository{db: db, queryTimeout: queryTimeout}
}

func (r *MySQLRepository) Create(ctx context.Context, input CreatePlanInput) (Plan, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	result, err := r.db.ExecContext(queryCtx, `
INSERT INTO care_plans (owner_id, task_type, title, local_time, enabled)
VALUES (?, ?, ?, ?, ?)`, input.OwnerID, input.TaskType, input.Title, input.LocalTime, enabled)
	if err != nil {
		return Plan{}, fmt.Errorf("create care plan: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Plan{}, fmt.Errorf("get care plan id: %w", err)
	}
	return r.get(queryCtx, input.OwnerID, uint64(id))
}

func (r *MySQLRepository) List(ctx context.Context, ownerID uint64) ([]Plan, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(queryCtx, `
SELECT id, owner_id, task_type, title, recurrence, local_time, enabled, created_at, updated_at
FROM care_plans
WHERE owner_id = ? AND deleted_at IS NULL
ORDER BY id`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list care plans: %w", err)
	}
	defer rows.Close()
	return scanPlans(rows)
}

func (r *MySQLRepository) ListEnabled(ctx context.Context, ownerID uint64) ([]Plan, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(queryCtx, `
SELECT id, owner_id, task_type, title, recurrence, local_time, enabled, created_at, updated_at
FROM care_plans
WHERE owner_id = ? AND enabled = TRUE AND deleted_at IS NULL
ORDER BY id`, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list enabled care plans: %w", err)
	}
	defer rows.Close()
	return scanPlans(rows)
}

func (r *MySQLRepository) Update(ctx context.Context, input UpdatePlanInput) (Plan, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	titlePresent, title := optionalString(input.Title)
	localTimePresent, localTime := optionalString(input.LocalTime)
	enabledPresent, enabled := optionalBool(input.Enabled)
	result, err := r.db.ExecContext(queryCtx, `
UPDATE care_plans
SET title = IF(?, ?, title),
    local_time = IF(?, ?, local_time),
    enabled = IF(?, ?, enabled)
WHERE id = ? AND owner_id = ? AND deleted_at IS NULL`,
		titlePresent, title,
		localTimePresent, localTime,
		enabledPresent, enabled,
		input.PlanID, input.OwnerID,
	)
	if err != nil {
		return Plan{}, fmt.Errorf("update care plan: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Plan{}, fmt.Errorf("read update care plan result: %w", err)
	}
	if affected == 0 {
		return r.get(queryCtx, input.OwnerID, input.PlanID)
	}
	return r.get(queryCtx, input.OwnerID, input.PlanID)
}

func (r *MySQLRepository) Delete(ctx context.Context, ownerID, planID uint64) error {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	result, err := r.db.ExecContext(queryCtx, `
UPDATE care_plans SET deleted_at = CURRENT_TIMESTAMP(6), enabled = FALSE
WHERE id = ? AND owner_id = ? AND deleted_at IS NULL`, planID, ownerID)
	if err != nil {
		return fmt.Errorf("delete care plan: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read delete care plan result: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLRepository) get(ctx context.Context, ownerID, planID uint64) (Plan, error) {
	var plan Plan
	err := r.db.QueryRowContext(ctx, `
SELECT id, owner_id, task_type, title, recurrence, local_time, enabled, created_at, updated_at
FROM care_plans
WHERE id = ? AND owner_id = ? AND deleted_at IS NULL`, planID, ownerID).Scan(
		&plan.ID, &plan.OwnerID, &plan.TaskType, &plan.Title, &plan.Recurrence, &plan.LocalTime, &plan.Enabled, &plan.CreatedAt, &plan.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Plan{}, ErrNotFound
	}
	if err != nil {
		return Plan{}, fmt.Errorf("get care plan: %w", err)
	}
	plan.CreatedAt = plan.CreatedAt.UTC()
	plan.UpdatedAt = plan.UpdatedAt.UTC()
	return plan, nil
}

func (r *MySQLRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := r.queryTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func scanPlans(rows *sql.Rows) ([]Plan, error) {
	plans := []Plan{}
	for rows.Next() {
		var plan Plan
		if err := rows.Scan(&plan.ID, &plan.OwnerID, &plan.TaskType, &plan.Title, &plan.Recurrence, &plan.LocalTime, &plan.Enabled, &plan.CreatedAt, &plan.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan care plan: %w", err)
		}
		plan.CreatedAt = plan.CreatedAt.UTC()
		plan.UpdatedAt = plan.UpdatedAt.UTC()
		plans = append(plans, plan)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate care plans: %w", err)
	}
	return plans, nil
}

func optionalString(value *string) (bool, string) {
	if value == nil {
		return false, ""
	}
	return true, *value
}

func optionalBool(value *bool) (bool, bool) {
	if value == nil {
		return false, false
	}
	return true, *value
}
