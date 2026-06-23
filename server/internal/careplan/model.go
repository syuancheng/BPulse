package careplan

import "time"

const (
	TaskTypeBloodPressure              = "blood_pressure"
	TaskTypeExercise                   = "exercise"
	TaskTypeDiet                       = "diet"
	TaskTypeMedicationReminderOptional = "medication_reminder_optional"
)

var allowedTaskTypes = map[string]struct{}{
	TaskTypeBloodPressure:              {},
	TaskTypeExercise:                   {},
	TaskTypeDiet:                       {},
	TaskTypeMedicationReminderOptional: {},
}

type Plan struct {
	ID         uint64
	OwnerID    uint64
	TaskType   string
	Title      string
	Recurrence string
	LocalTime  string
	Enabled    bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CreatePlanInput struct {
	OwnerID   uint64
	TaskType  string
	Title     string
	LocalTime string
	Enabled   *bool
}

type UpdatePlanInput struct {
	OwnerID   uint64
	PlanID    uint64
	Title     *string
	LocalTime *string
	Enabled   *bool
}
