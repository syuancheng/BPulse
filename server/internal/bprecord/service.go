package bprecord

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	minSystolic  = 40
	maxSystolic  = 260
	minDiastolic = 30
	maxDiastolic = 180
	minPulse     = 30
	maxPulse     = 220
	defaultLimit = 20
	maxLimit     = 100
)

var clientRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,80}$`)

type Repository interface {
	CreateOrGet(ctx context.Context, record Record) (Record, bool, error)
	List(ctx context.Context, filter ListFilter) ([]Record, error)
	Get(ctx context.Context, ownerID, recordID uint64) (Record, error)
}

type Service struct {
	repository Repository
	keyring    Keyring
	clock      Clock
}

func NewService(repository Repository, keyring Keyring, clock Clock) *Service {
	if clock == nil {
		clock = RealClock{}
	}
	return &Service{repository: repository, keyring: keyring, clock: clock}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Record, error) {
	measuredAtUTC, payload, err := s.validateAndBuild(input)
	if err != nil {
		return Record{}, err
	}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return Record{}, fmt.Errorf("marshal blood pressure payload: %w", err)
	}
	aad := additionalData(input.OwnerID, input.ClientRequestID, measuredAtUTC, input.EntryMethod)
	keyVersion, nonce, ciphertext, err := s.keyring.Encrypt(plaintext, aad)
	if err != nil {
		return Record{}, err
	}
	record := Record{
		OwnerID:         input.OwnerID,
		ClientRequestID: input.ClientRequestID,
		MeasuredAtUTC:   measuredAtUTC,
		Timezone:        input.Timezone,
		EntryMethod:     input.EntryMethod,
		KeyVersion:      keyVersion,
		Nonce:           nonce,
		Ciphertext:      ciphertext,
		Payload:         payload,
	}
	stored, existed, err := s.repository.CreateOrGet(ctx, record)
	if err != nil {
		return Record{}, err
	}
	if err := s.attachPayload(&stored); err != nil {
		return Record{}, err
	}
	if existed && !sameRecordShape(stored, record) {
		return Record{}, ErrConflict
	}
	return stored, nil
}

func (s *Service) List(ctx context.Context, filter ListFilter) ([]Record, error) {
	if filter.OwnerID == 0 {
		return nil, fmt.Errorf("%w: owner is required", ErrInvalid)
	}
	if filter.Limit <= 0 {
		filter.Limit = defaultLimit
	}
	if filter.Limit > maxLimit {
		filter.Limit = maxLimit
	}
	records, err := s.repository.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	for i := range records {
		if err := s.attachPayload(&records[i]); err != nil {
			return nil, err
		}
	}
	return records, nil
}

func (s *Service) Get(ctx context.Context, ownerID, recordID uint64) (Record, error) {
	if ownerID == 0 || recordID == 0 {
		return Record{}, fmt.Errorf("%w: owner and record are required", ErrInvalid)
	}
	record, err := s.repository.Get(ctx, ownerID, recordID)
	if err != nil {
		return Record{}, err
	}
	if err := s.attachPayload(&record); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Service) Trends(ctx context.Context, ownerID uint64, timezone string, days int) ([]TrendPoint, error) {
	if days <= 0 {
		days = 7
	}
	if days > 31 {
		days = 31
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, fmt.Errorf("%w: timezone is invalid", ErrInvalid)
	}
	nowLocal := s.clock.Now().In(location)
	startLocal := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -days+1)
	endLocal := startLocal.AddDate(0, 0, days)
	fromUTC := startLocal.UTC()
	toUTC := endLocal.UTC()
	records, err := s.List(ctx, ListFilter{OwnerID: ownerID, FromUTC: &fromUTC, ToUTC: &toUTC, Limit: maxLimit})
	if err != nil {
		return nil, err
	}
	buckets := map[string][]SummaryPayload{}
	for _, record := range records {
		day := record.MeasuredAtUTC.In(location).Format("2006-01-02")
		buckets[day] = append(buckets[day], record.Payload.Summary)
	}
	daysWithRecords := make([]string, 0, len(buckets))
	for day := range buckets {
		daysWithRecords = append(daysWithRecords, day)
	}
	sort.Strings(daysWithRecords)
	points := make([]TrendPoint, 0, len(daysWithRecords))
	for _, day := range daysWithRecords {
		summaries := buckets[day]
		var sysTotal, diaTotal, pulseTotal, pulseCount int
		for _, summary := range summaries {
			sysTotal += summary.AverageSystolic
			diaTotal += summary.AverageDiastolic
			if summary.AveragePulse != nil {
				pulseTotal += *summary.AveragePulse
				pulseCount++
			}
		}
		point := TrendPoint{
			LocalDate:        day,
			AverageSystolic:  roundedAverage(sysTotal, len(summaries)),
			AverageDiastolic: roundedAverage(diaTotal, len(summaries)),
			RecordCount:      len(summaries),
		}
		if pulseCount > 0 {
			value := roundedAverage(pulseTotal, pulseCount)
			point.AveragePulse = &value
		}
		points = append(points, point)
	}
	return points, nil
}

func (s *Service) validateAndBuild(input CreateInput) (time.Time, Payload, error) {
	if input.OwnerID == 0 {
		return time.Time{}, Payload{}, fmt.Errorf("%w: owner is required", ErrInvalid)
	}
	if !clientRequestIDPattern.MatchString(input.ClientRequestID) {
		return time.Time{}, Payload{}, fmt.Errorf("%w: client request id is invalid", ErrInvalid)
	}
	if input.EntryMethod != EntryMethodManual && input.EntryMethod != EntryMethodVoice && input.EntryMethod != EntryMethodPhoto {
		return time.Time{}, Payload{}, fmt.Errorf("%w: entry method is invalid", ErrInvalid)
	}
	location, err := time.LoadLocation(input.Timezone)
	if err != nil {
		return time.Time{}, Payload{}, fmt.Errorf("%w: timezone is invalid", ErrInvalid)
	}
	measuredAt, err := time.Parse(time.RFC3339, input.MeasuredAt)
	if err != nil {
		return time.Time{}, Payload{}, fmt.Errorf("%w: measuredAt must be RFC3339", ErrInvalid)
	}
	measuredAtLocal := measuredAt.In(location)
	_, measuredOffset := measuredAt.Zone()
	_, locationOffset := measuredAtLocal.Zone()
	if measuredOffset != locationOffset {
		return time.Time{}, Payload{}, fmt.Errorf("%w: measuredAt offset does not match timezone", ErrInvalid)
	}
	now := s.clock.Now().UTC()
	if (measuredAt.UTC().After(now.Add(5*time.Minute)) || measuredAt.UTC().Before(now.AddDate(0, 0, -30))) && !input.TimeConfirmed {
		return time.Time{}, Payload{}, fmt.Errorf("%w: measurement time requires confirmation", ErrInvalid)
	}
	if len(input.Readings) < 1 || len(input.Readings) > 2 {
		return time.Time{}, Payload{}, fmt.Errorf("%w: one or two readings are required", ErrInvalid)
	}
	readings := make([]ReadingPayload, 0, len(input.Readings))
	for _, reading := range input.Readings {
		if reading.Systolic < minSystolic || reading.Systolic > maxSystolic || reading.Diastolic < minDiastolic || reading.Diastolic > maxDiastolic || reading.Diastolic >= reading.Systolic {
			return time.Time{}, Payload{}, fmt.Errorf("%w: reading is out of data quality bounds", ErrInvalid)
		}
		if reading.Pulse != nil && (*reading.Pulse < minPulse || *reading.Pulse > maxPulse) {
			return time.Time{}, Payload{}, fmt.Errorf("%w: pulse is out of data quality bounds", ErrInvalid)
		}
		readings = append(readings, ReadingPayload(reading))
	}
	var note *string
	if input.Note != nil {
		trimmed := strings.TrimSpace(*input.Note)
		if len([]rune(trimmed)) > 200 {
			return time.Time{}, Payload{}, fmt.Errorf("%w: note is too long", ErrInvalid)
		}
		if trimmed != "" {
			note = &trimmed
		}
	}
	return measuredAt.UTC(), Payload{Readings: readings, Summary: summarize(readings), Note: note}, nil
}

func (s *Service) attachPayload(record *Record) error {
	aad := additionalData(record.OwnerID, record.ClientRequestID, record.MeasuredAtUTC, record.EntryMethod)
	plaintext, err := s.keyring.Decrypt(record.KeyVersion, record.Nonce, record.Ciphertext, aad)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(plaintext, &record.Payload); err != nil {
		return ErrDecryptFailed
	}
	return nil
}

func summarize(readings []ReadingPayload) SummaryPayload {
	var sysTotal, diaTotal, pulseTotal, pulseCount int
	for _, reading := range readings {
		sysTotal += reading.Systolic
		diaTotal += reading.Diastolic
		if reading.Pulse != nil {
			pulseTotal += *reading.Pulse
			pulseCount++
		}
	}
	summary := SummaryPayload{
		AverageSystolic:  roundedAverage(sysTotal, len(readings)),
		AverageDiastolic: roundedAverage(diaTotal, len(readings)),
		ReadingCount:     len(readings),
	}
	if pulseCount > 0 {
		value := roundedAverage(pulseTotal, pulseCount)
		summary.AveragePulse = &value
	}
	return summary
}

func roundedAverage(total, count int) int {
	return int(math.Round(float64(total) / float64(count)))
}

func additionalData(ownerID uint64, clientRequestID string, measuredAtUTC time.Time, entryMethod string) []byte {
	return []byte(fmt.Sprintf("%d|%s|%s|%s", ownerID, clientRequestID, measuredAtUTC.UTC().Format(time.RFC3339Nano), entryMethod))
}

func sameRecordShape(a, b Record) bool {
	return a.ClientRequestID == b.ClientRequestID &&
		a.MeasuredAtUTC.Equal(b.MeasuredAtUTC) &&
		a.Timezone == b.Timezone &&
		a.EntryMethod == b.EntryMethod &&
		reflect.DeepEqual(a.Payload, b.Payload)
}
