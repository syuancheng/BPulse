package task

import (
	"context"
	"testing"
	"time"

	"github.com/syuancheng/BPulse/server/internal/careplan"
)

type fakeClock struct{ now time.Time }

func (c fakeClock) Now() time.Time { return c.now }

type fakePlanRepository struct{ plans []careplan.Plan }

func (r fakePlanRepository) ListEnabled(context.Context, uint64) ([]careplan.Plan, error) {
	return r.plans, nil
}

type fakeTaskRepository struct {
	localDate string
	location  string
	plans     []careplan.Plan
}

func (r *fakeTaskRepository) CreateForPlans(_ context.Context, _ uint64, localDate string, location *time.Location, plans []careplan.Plan) error {
	r.localDate = localDate
	r.location = location.String()
	r.plans = plans
	return nil
}

func (r *fakeTaskRepository) ListForLocalDate(context.Context, uint64, string) ([]Instance, error) {
	return []Instance{{ID: 1, ScheduledLocalDate: r.localDate, State: StatePending}}, nil
}

func (r *fakeTaskRepository) SetState(_ context.Context, _ uint64, taskID uint64, state string, nowUTC time.Time) (Instance, error) {
	return Instance{ID: taskID, State: state, ScheduledAtUTC: nowUTC}, nil
}

func TestTodayUsesUserTimezoneBoundary(t *testing.T) {
	repository := &fakeTaskRepository{}
	clock := fakeClock{now: time.Date(2026, 6, 22, 17, 30, 0, 0, time.UTC)}
	plans := fakePlanRepository{plans: []careplan.Plan{{ID: 7, LocalTime: "07:00"}}}

	instances, err := NewService(plans, repository, clock).Today(context.Background(), 10, "Asia/Singapore")
	if err != nil {
		t.Fatalf("Today() error = %v", err)
	}
	if repository.localDate != "2026-06-23" || repository.location != "Asia/Singapore" {
		t.Fatalf("local date/location = %s/%s", repository.localDate, repository.location)
	}
	if len(repository.plans) != 1 || len(instances) != 1 {
		t.Fatalf("plans=%d instances=%d", len(repository.plans), len(instances))
	}
}

func TestCompleteUsesClock(t *testing.T) {
	repository := &fakeTaskRepository{}
	now := time.Date(2026, 6, 23, 1, 2, 3, 0, time.UTC)
	instance, err := NewService(fakePlanRepository{}, repository, fakeClock{now: now}).Complete(context.Background(), 9, 11)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if instance.State != StateCompleted || !instance.ScheduledAtUTC.Equal(now) {
		t.Fatalf("instance = %#v", instance)
	}
}
