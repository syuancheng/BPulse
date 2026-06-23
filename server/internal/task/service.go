package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/syuancheng/BPulse/server/internal/careplan"
)

var (
	ErrInvalid  = errors.New("invalid task")
	ErrNotFound = errors.New("task not found")
)

type PlanRepository interface {
	ListEnabled(ctx context.Context, ownerID uint64) ([]careplan.Plan, error)
}

type Service struct {
	plans      PlanRepository
	repository Repository
	clock      Clock
}

func NewService(plans PlanRepository, repository Repository, clock Clock) *Service {
	if clock == nil {
		clock = RealClock{}
	}
	return &Service{plans: plans, repository: repository, clock: clock}
}

func (s *Service) Today(ctx context.Context, ownerID uint64, timezone string) ([]Instance, error) {
	location, localDate, err := s.localDay(timezone)
	if err != nil {
		return nil, err
	}
	plans, err := s.plans.ListEnabled(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list enabled plans for tasks: %w", err)
	}
	if err := s.repository.CreateForPlans(ctx, ownerID, localDate, location, plans); err != nil {
		return nil, fmt.Errorf("generate today's tasks: %w", err)
	}
	return s.repository.ListForLocalDate(ctx, ownerID, localDate)
}

func (s *Service) Complete(ctx context.Context, ownerID, taskID uint64) (Instance, error) {
	return s.setState(ctx, ownerID, taskID, StateCompleted)
}

func (s *Service) Skip(ctx context.Context, ownerID, taskID uint64) (Instance, error) {
	return s.setState(ctx, ownerID, taskID, StateSkipped)
}

func (s *Service) setState(ctx context.Context, ownerID, taskID uint64, state string) (Instance, error) {
	if ownerID == 0 || taskID == 0 {
		return Instance{}, fmt.Errorf("%w: owner and task are required", ErrInvalid)
	}
	instance, err := s.repository.SetState(ctx, ownerID, taskID, state, s.clock.Now().UTC())
	if err != nil {
		return Instance{}, err
	}
	return instance, nil
}

func (s *Service) localDay(timezone string) (*time.Location, string, error) {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, "", fmt.Errorf("%w: timezone is invalid", ErrInvalid)
	}
	nowLocal := s.clock.Now().In(location)
	return location, nowLocal.Format("2006-01-02"), nil
}
