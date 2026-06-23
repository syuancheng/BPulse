package user

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository interface {
	UpsertAndGet(ctx context.Context, identityReference [32]byte) (Profile, error)
	UpdatePreferences(ctx context.Context, identityReference [32]byte, patch PreferencesPatch) (Profile, error)
}

type MySQLRepository struct {
	db           *sql.DB
	queryTimeout time.Duration
}

func NewMySQLRepository(db *sql.DB, queryTimeout time.Duration) *MySQLRepository {
	return &MySQLRepository{db: db, queryTimeout: queryTimeout}
}

func (r *MySQLRepository) UpsertAndGet(ctx context.Context, identityReference [32]byte) (Profile, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	if _, err := r.db.ExecContext(queryCtx, `
INSERT INTO users (openid_hash) VALUES (?)
ON DUPLICATE KEY UPDATE openid_hash = openid_hash`, identityReference[:]); err != nil {
		return Profile{}, fmt.Errorf("upsert user: %w", err)
	}
	return r.getByIdentity(queryCtx, identityReference)
}

func (r *MySQLRepository) UpdatePreferences(ctx context.Context, identityReference [32]byte, patch PreferencesPatch) (Profile, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	timezonePresent, timezone := optionalString(patch.Timezone)
	largeTextPresent, largeText := optionalBool(patch.LargeTextEnabled)
	highContrastPresent, highContrast := optionalBool(patch.HighContrastEnabled)
	if _, err := r.db.ExecContext(queryCtx, `
UPDATE users
SET timezone = IF(?, ?, timezone),
    large_text_enabled = IF(?, ?, large_text_enabled),
    high_contrast_enabled = IF(?, ?, high_contrast_enabled)
WHERE openid_hash = ?`,
		timezonePresent, timezone,
		largeTextPresent, largeText,
		highContrastPresent, highContrast,
		identityReference[:],
	); err != nil {
		return Profile{}, fmt.Errorf("update user preferences: %w", err)
	}
	return r.getByIdentity(queryCtx, identityReference)
}

func (r *MySQLRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := r.queryTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func (r *MySQLRepository) getByIdentity(ctx context.Context, identityReference [32]byte) (Profile, error) {
	var profile Profile
	err := r.db.QueryRowContext(ctx, `
SELECT id, timezone, large_text_enabled, high_contrast_enabled, created_at, updated_at
FROM users
WHERE openid_hash = ?`, identityReference[:]).Scan(
		&profile.ID,
		&profile.Timezone,
		&profile.LargeTextEnabled,
		&profile.HighContrastEnabled,
		&profile.CreatedAt,
		&profile.UpdatedAt,
	)
	if err != nil {
		return Profile{}, fmt.Errorf("get user by identity: %w", err)
	}
	profile.CreatedAt = profile.CreatedAt.UTC()
	profile.UpdatedAt = profile.UpdatedAt.UTC()
	return profile, nil
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
