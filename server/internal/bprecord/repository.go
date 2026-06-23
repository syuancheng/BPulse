package bprecord

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type MySQLRepository struct {
	db           *sql.DB
	queryTimeout time.Duration
}

func NewMySQLRepository(db *sql.DB, queryTimeout time.Duration) *MySQLRepository {
	return &MySQLRepository{db: db, queryTimeout: queryTimeout}
}

func (r *MySQLRepository) CreateOrGet(ctx context.Context, record Record) (Record, bool, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	result, err := r.db.ExecContext(queryCtx, `
INSERT IGNORE INTO bp_records (owner_id, client_request_id, measured_at_utc, timezone, entry_method, key_version, nonce, ciphertext)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.OwnerID, record.ClientRequestID, record.MeasuredAtUTC.UTC(), record.Timezone, record.EntryMethod, record.KeyVersion, record.Nonce, record.Ciphertext,
	)
	if err != nil {
		return Record{}, false, fmt.Errorf("create blood pressure record: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Record{}, false, fmt.Errorf("read blood pressure insert result: %w", err)
	}
	stored, err := r.getByClientRequestID(queryCtx, record.OwnerID, record.ClientRequestID)
	if err != nil {
		return Record{}, false, err
	}
	return stored, affected == 0, nil
}

func (r *MySQLRepository) List(ctx context.Context, filter ListFilter) ([]Record, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.QueryContext(queryCtx, `
SELECT id, owner_id, client_request_id, measured_at_utc, timezone, entry_method, key_version, nonce, ciphertext, created_at, updated_at
FROM bp_records
WHERE owner_id = ?
  AND (? IS NULL OR measured_at_utc >= ?)
  AND (? IS NULL OR measured_at_utc < ?)
ORDER BY measured_at_utc DESC, id DESC
LIMIT ?`, filter.OwnerID, filter.FromUTC, filter.FromUTC, filter.ToUTC, filter.ToUTC, limit)
	if err != nil {
		return nil, fmt.Errorf("list blood pressure records: %w", err)
	}
	defer rows.Close()
	return scanRecords(rows)
}

func (r *MySQLRepository) Get(ctx context.Context, ownerID, recordID uint64) (Record, error) {
	queryCtx, cancel := r.withTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(queryCtx, `
SELECT id, owner_id, client_request_id, measured_at_utc, timezone, entry_method, key_version, nonce, ciphertext, created_at, updated_at
FROM bp_records
WHERE id = ? AND owner_id = ?`, recordID, ownerID)
	if err != nil {
		return Record{}, fmt.Errorf("get blood pressure record: %w", err)
	}
	defer rows.Close()
	records, err := scanRecords(rows)
	if err != nil {
		return Record{}, err
	}
	if len(records) == 0 {
		return Record{}, ErrNotFound
	}
	return records[0], nil
}

func (r *MySQLRepository) getByClientRequestID(ctx context.Context, ownerID uint64, clientRequestID string) (Record, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT id, owner_id, client_request_id, measured_at_utc, timezone, entry_method, key_version, nonce, ciphertext, created_at, updated_at
FROM bp_records
WHERE owner_id = ? AND client_request_id = ?`, ownerID, clientRequestID)
	if err != nil {
		return Record{}, fmt.Errorf("get blood pressure record by client request id: %w", err)
	}
	defer rows.Close()
	records, err := scanRecords(rows)
	if err != nil {
		return Record{}, err
	}
	if len(records) == 0 {
		return Record{}, ErrNotFound
	}
	return records[0], nil
}

func (r *MySQLRepository) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := r.queryTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func scanRecords(rows *sql.Rows) ([]Record, error) {
	records := []Record{}
	for rows.Next() {
		var record Record
		if err := rows.Scan(&record.ID, &record.OwnerID, &record.ClientRequestID, &record.MeasuredAtUTC, &record.Timezone, &record.EntryMethod, &record.KeyVersion, &record.Nonce, &record.Ciphertext, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan blood pressure record: %w", err)
		}
		record.MeasuredAtUTC = record.MeasuredAtUTC.UTC()
		record.CreatedAt = record.CreatedAt.UTC()
		record.UpdatedAt = record.UpdatedAt.UTC()
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate blood pressure records: %w", err)
	}
	return records, nil
}
