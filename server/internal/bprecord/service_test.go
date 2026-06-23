package bprecord

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type memoryRepository struct {
	records []Record
	nextID  uint64
}

func (r *memoryRepository) CreateOrGet(_ context.Context, record Record) (Record, bool, error) {
	for _, existing := range r.records {
		if existing.OwnerID == record.OwnerID && existing.ClientRequestID == record.ClientRequestID {
			return existing, true, nil
		}
	}
	r.nextID++
	record.ID = r.nextID
	record.CreatedAt = time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	record.UpdatedAt = record.CreatedAt
	r.records = append(r.records, record)
	return record, false, nil
}

func (r *memoryRepository) List(_ context.Context, filter ListFilter) ([]Record, error) {
	var out []Record
	for _, record := range r.records {
		if record.OwnerID != filter.OwnerID {
			continue
		}
		if filter.FromUTC != nil && record.MeasuredAtUTC.Before(*filter.FromUTC) {
			continue
		}
		if filter.ToUTC != nil && !record.MeasuredAtUTC.Before(*filter.ToUTC) {
			continue
		}
		out = append(out, record)
	}
	return out, nil
}

func (r *memoryRepository) Get(_ context.Context, ownerID, recordID uint64) (Record, error) {
	for _, record := range r.records {
		if record.OwnerID == ownerID && record.ID == recordID {
			return record, nil
		}
	}
	return Record{}, ErrNotFound
}

func TestCreateEncryptsComputesAverageAndIsIdempotent(t *testing.T) {
	service, repository := newTestService(t)
	pulseOne := 70
	pulseTwo := 72
	note := "  synthetic note  "
	input := CreateInput{
		OwnerID:         1,
		ClientRequestID: "req-12345678",
		MeasuredAt:      "2026-06-23T18:30:00+08:00",
		Timezone:        "Asia/Singapore",
		EntryMethod:     EntryMethodManual,
		Readings: []ReadingInput{
			{Systolic: 131, Diastolic: 83, Pulse: &pulseOne},
			{Systolic: 133, Diastolic: 85, Pulse: &pulseTwo},
		},
		Note: &note,
	}

	first, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if first.MeasuredAtUTC.Format(time.RFC3339) != "2026-06-23T10:30:00Z" {
		t.Fatalf("MeasuredAtUTC = %s", first.MeasuredAtUTC.Format(time.RFC3339))
	}
	if first.Payload.Summary.AverageSystolic != 132 || first.Payload.Summary.AverageDiastolic != 84 || *first.Payload.Summary.AveragePulse != 71 {
		t.Fatalf("summary = %#v", first.Payload.Summary)
	}
	if first.Payload.Note == nil || *first.Payload.Note != "synthetic note" {
		t.Fatalf("note = %#v", first.Payload.Note)
	}
	if len(repository.records) != 1 || string(repository.records[0].Ciphertext) == "" || string(repository.records[0].Ciphertext) == "132" {
		t.Fatalf("stored ciphertext looks invalid: %#v", repository.records[0].Ciphertext)
	}

	second, err := service.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("repeated Create() error = %v", err)
	}
	if second.ID != first.ID || len(repository.records) != 1 {
		t.Fatalf("idempotent save created duplicate: first=%d second=%d count=%d", first.ID, second.ID, len(repository.records))
	}
}

func TestCreateDetectsIdempotencyConflict(t *testing.T) {
	service, _ := newTestService(t)
	input := validInput()
	if _, err := service.Create(context.Background(), input); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	input.Readings[0].Systolic = 140
	if _, err := service.Create(context.Background(), input); !errors.Is(err, ErrConflict) {
		t.Fatalf("Create() conflict error = %v, want ErrConflict", err)
	}
}

func TestValidationRejectsBadBoundsAndUnconfirmedOldTime(t *testing.T) {
	service, _ := newTestService(t)
	input := validInput()
	input.Readings[0].Diastolic = 200
	if _, err := service.Create(context.Background(), input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Create() invalid bounds error = %v", err)
	}
	input = validInput()
	input.MeasuredAt = "2026-05-01T10:00:00Z"
	input.Timezone = "UTC"
	if _, err := service.Create(context.Background(), input); !errors.Is(err, ErrInvalid) {
		t.Fatalf("Create() old time error = %v", err)
	}
	input.TimeConfirmed = true
	if _, err := service.Create(context.Background(), input); err != nil {
		t.Fatalf("Create() confirmed old time error = %v", err)
	}
}

func TestNonceUniquenessAndWrongKeyFailure(t *testing.T) {
	service, repository := newTestService(t)
	input := validInput()
	if _, err := service.Create(context.Background(), input); err != nil {
		t.Fatalf("Create() first error = %v", err)
	}
	input.ClientRequestID = "req-87654321"
	if _, err := service.Create(context.Background(), input); err != nil {
		t.Fatalf("Create() second error = %v", err)
	}
	if string(repository.records[0].Nonce) == string(repository.records[1].Nonce) {
		t.Fatal("nonce was reused")
	}
	wrongKey := base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))
	keyring, err := NewKeyringFromBase64("test-v1", wrongKey)
	if err != nil {
		t.Fatalf("NewKeyringFromBase64() error = %v", err)
	}
	wrongService := NewService(repository, keyring, fixedClock{now: testNow()})
	if _, err := wrongService.Get(context.Background(), 1, 1); !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("wrong key Get() error = %v, want ErrDecryptFailed", err)
	}
	repository.records[0].Ciphertext[0] ^= 0x01
	if _, err := service.Get(context.Background(), 1, 1); !errors.Is(err, ErrDecryptFailed) {
		t.Fatalf("corrupted ciphertext Get() error = %v, want ErrDecryptFailed", err)
	}
}

func TestTrendsUsesUserTimezoneBoundaries(t *testing.T) {
	service, _ := newTestService(t)
	first := validInput()
	first.ClientRequestID = "req-11111111"
	first.MeasuredAt = "2026-06-22T23:30:00-07:00"
	first.Timezone = "America/Los_Angeles"
	if _, err := service.Create(context.Background(), first); err != nil {
		t.Fatalf("Create() first error = %v", err)
	}
	second := validInput()
	second.ClientRequestID = "req-22222222"
	second.MeasuredAt = "2026-06-23T00:30:00-07:00"
	second.Timezone = "America/Los_Angeles"
	if _, err := service.Create(context.Background(), second); err != nil {
		t.Fatalf("Create() second error = %v", err)
	}
	points, err := service.Trends(context.Background(), 1, "America/Los_Angeles", 3)
	if err != nil {
		t.Fatalf("Trends() error = %v", err)
	}
	if len(points) != 2 || points[0].LocalDate != "2026-06-22" || points[1].LocalDate != "2026-06-23" {
		t.Fatalf("points = %#v", points)
	}
}

func newTestService(t *testing.T) (*Service, *memoryRepository) {
	t.Helper()
	keyring, err := NewKeyringFromBase64("test-v1", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatalf("NewKeyringFromBase64() error = %v", err)
	}
	repository := &memoryRepository{}
	return NewService(repository, keyring, fixedClock{now: testNow()}), repository
}

func validInput() CreateInput {
	pulse := 70
	return CreateInput{
		OwnerID:         1,
		ClientRequestID: "req-12345678",
		MeasuredAt:      "2026-06-23T18:30:00+08:00",
		Timezone:        "Asia/Singapore",
		EntryMethod:     EntryMethodManual,
		Readings:        []ReadingInput{{Systolic: 132, Diastolic: 84, Pulse: &pulse}},
	}
}

func testNow() time.Time {
	return time.Date(2026, 6, 23, 11, 0, 0, 0, time.UTC)
}
