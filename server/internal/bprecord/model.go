package bprecord

import "time"

const (
	EntryMethodManual = "manual"
	EntryMethodVoice  = "voice"
	EntryMethodPhoto  = "photo"
)

type ReadingInput struct {
	Systolic  int  `json:"systolic"`
	Diastolic int  `json:"diastolic"`
	Pulse     *int `json:"pulse,omitempty"`
}

type CreateInput struct {
	OwnerID         uint64
	ClientRequestID string
	MeasuredAt      string
	Timezone        string
	EntryMethod     string
	Readings        []ReadingInput
	Note            *string
	TimeConfirmed   bool
}

type Record struct {
	ID              uint64
	OwnerID         uint64
	ClientRequestID string
	MeasuredAtUTC   time.Time
	Timezone        string
	EntryMethod     string
	KeyVersion      string
	Nonce           []byte
	Ciphertext      []byte
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Payload         Payload
}

type Payload struct {
	Readings []ReadingPayload `json:"readings"`
	Summary  SummaryPayload   `json:"summary"`
	Note     *string          `json:"note,omitempty"`
}

type ReadingPayload struct {
	Systolic  int  `json:"systolic"`
	Diastolic int  `json:"diastolic"`
	Pulse     *int `json:"pulse,omitempty"`
}

type SummaryPayload struct {
	AverageSystolic  int  `json:"averageSystolic"`
	AverageDiastolic int  `json:"averageDiastolic"`
	AveragePulse     *int `json:"averagePulse,omitempty"`
	ReadingCount     int  `json:"readingCount"`
}

type ListFilter struct {
	OwnerID uint64
	FromUTC *time.Time
	ToUTC   *time.Time
	Limit   int
}

type TrendPoint struct {
	LocalDate        string
	AverageSystolic  int
	AverageDiastolic int
	AveragePulse     *int
	RecordCount      int
}

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }
