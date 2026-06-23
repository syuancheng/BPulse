package careplan

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct{}

func (fakeRepository) Create(context.Context, CreatePlanInput) (Plan, error) { return Plan{}, nil }
func (fakeRepository) List(context.Context, uint64) ([]Plan, error)          { return nil, nil }
func (fakeRepository) Update(context.Context, UpdatePlanInput) (Plan, error) { return Plan{}, nil }
func (fakeRepository) Delete(context.Context, uint64, uint64) error          { return nil }
func (fakeRepository) ListEnabled(context.Context, uint64) ([]Plan, error)   { return nil, nil }

func TestCreateValidation(t *testing.T) {
	enabled := true
	tests := []struct {
		name    string
		input   CreatePlanInput
		wantErr bool
	}{
		{name: "valid blood pressure", input: CreatePlanInput{OwnerID: 1, TaskType: TaskTypeBloodPressure, Title: "测血压", LocalTime: "07:30", Enabled: &enabled}},
		{name: "valid optional medication reminder", input: CreatePlanInput{OwnerID: 1, TaskType: TaskTypeMedicationReminderOptional, Title: "用药提醒", LocalTime: "08:00"}},
		{name: "missing owner", input: CreatePlanInput{TaskType: TaskTypeDiet, Title: "饮食记录", LocalTime: "12:00"}, wantErr: true},
		{name: "bad type", input: CreatePlanInput{OwnerID: 1, TaskType: "diagnosis", Title: "诊断", LocalTime: "12:00"}, wantErr: true},
		{name: "bad time", input: CreatePlanInput{OwnerID: 1, TaskType: TaskTypeExercise, Title: "运动", LocalTime: "24:00"}, wantErr: true},
		{name: "empty title", input: CreatePlanInput{OwnerID: 1, TaskType: TaskTypeDiet, Title: " ", LocalTime: "12:00"}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(fakeRepository{}).Create(context.Background(), test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("Create() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantErr && !errors.Is(err, ErrInvalid) {
				t.Fatalf("Create() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestUpdateValidation(t *testing.T) {
	title := "新的饮食任务"
	badTime := "9:00"
	validTime := "09:00"
	tests := []struct {
		name    string
		input   UpdatePlanInput
		wantErr bool
	}{
		{name: "valid title", input: UpdatePlanInput{OwnerID: 1, PlanID: 2, Title: &title}},
		{name: "valid time", input: UpdatePlanInput{OwnerID: 1, PlanID: 2, LocalTime: &validTime}},
		{name: "missing fields", input: UpdatePlanInput{OwnerID: 1, PlanID: 2}, wantErr: true},
		{name: "bad time", input: UpdatePlanInput{OwnerID: 1, PlanID: 2, LocalTime: &badTime}, wantErr: true},
		{name: "missing plan", input: UpdatePlanInput{OwnerID: 1, Title: &title}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewService(fakeRepository{}).Update(context.Background(), test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("Update() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
