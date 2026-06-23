package careplan

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrInvalid  = errors.New("invalid care plan")
	ErrNotFound = errors.New("care plan not found")
)

var localTimePattern = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Create(ctx context.Context, input CreatePlanInput) (Plan, error) {
	if err := validateCreate(input); err != nil {
		return Plan{}, err
	}
	return s.repository.Create(ctx, input)
}

func (s *Service) List(ctx context.Context, ownerID uint64) ([]Plan, error) {
	return s.repository.List(ctx, ownerID)
}

func (s *Service) Update(ctx context.Context, input UpdatePlanInput) (Plan, error) {
	if err := validateUpdate(input); err != nil {
		return Plan{}, err
	}
	return s.repository.Update(ctx, input)
}

func (s *Service) Delete(ctx context.Context, ownerID, planID uint64) error {
	return s.repository.Delete(ctx, ownerID, planID)
}

func validateCreate(input CreatePlanInput) error {
	if input.OwnerID == 0 {
		return fmt.Errorf("%w: owner is required", ErrInvalid)
	}
	if _, ok := allowedTaskTypes[input.TaskType]; !ok {
		return fmt.Errorf("%w: task type is invalid", ErrInvalid)
	}
	if err := validateTitle(input.Title); err != nil {
		return err
	}
	if !localTimePattern.MatchString(input.LocalTime) {
		return fmt.Errorf("%w: local time must be HH:MM", ErrInvalid)
	}
	return nil
}

func validateUpdate(input UpdatePlanInput) error {
	if input.OwnerID == 0 || input.PlanID == 0 {
		return fmt.Errorf("%w: owner and plan are required", ErrInvalid)
	}
	if input.Title == nil && input.LocalTime == nil && input.Enabled == nil {
		return fmt.Errorf("%w: at least one field is required", ErrInvalid)
	}
	if input.Title != nil {
		if err := validateTitle(*input.Title); err != nil {
			return err
		}
	}
	if input.LocalTime != nil && !localTimePattern.MatchString(*input.LocalTime) {
		return fmt.Errorf("%w: local time must be HH:MM", ErrInvalid)
	}
	return nil
}

func validateTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" || len([]rune(title)) > 80 {
		return fmt.Errorf("%w: title is required", ErrInvalid)
	}
	return nil
}
