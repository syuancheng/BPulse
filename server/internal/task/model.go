package task

import "time"

const (
	StatePending   = "pending"
	StateCompleted = "completed"
	StateSkipped   = "skipped"
)

type Instance struct {
	ID                 uint64
	OwnerID            uint64
	CarePlanID         uint64
	TaskType           string
	Title              string
	ScheduledLocalDate string
	OccurrenceKey      string
	ScheduledAtUTC     time.Time
	State              string
	CompletedAtUTC     *time.Time
	SkippedAtUTC       *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }
