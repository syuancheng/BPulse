package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/syuancheng/BPulse/server/internal/auth"
)

var ErrInvalidPreferences = errors.New("invalid preferences")

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetOrCreate(ctx context.Context, identity auth.Identity) (Profile, error) {
	profile, err := s.repository.UpsertAndGet(ctx, identity.Reference)
	if err != nil {
		return Profile{}, fmt.Errorf("get or create current user: %w", err)
	}
	return profile, nil
}

func (s *Service) UpdatePreferences(ctx context.Context, identity auth.Identity, patch PreferencesPatch) (Profile, error) {
	if err := validatePatch(patch); err != nil {
		return Profile{}, err
	}
	if _, err := s.repository.UpsertAndGet(ctx, identity.Reference); err != nil {
		return Profile{}, fmt.Errorf("bootstrap current user: %w", err)
	}
	profile, err := s.repository.UpdatePreferences(ctx, identity.Reference, patch)
	if err != nil {
		return Profile{}, fmt.Errorf("update current user preferences: %w", err)
	}
	return profile, nil
}

func validatePatch(patch PreferencesPatch) error {
	if patch.Timezone == nil && patch.LargeTextEnabled == nil && patch.HighContrastEnabled == nil {
		return fmt.Errorf("%w: at least one preference is required", ErrInvalidPreferences)
	}
	if patch.Timezone != nil {
		if *patch.Timezone == "" || len(*patch.Timezone) > 64 || *patch.Timezone == "Local" {
			return fmt.Errorf("%w: timezone must be a valid IANA name", ErrInvalidPreferences)
		}
		if _, err := time.LoadLocation(*patch.Timezone); err != nil {
			return fmt.Errorf("%w: timezone must be a valid IANA name", ErrInvalidPreferences)
		}
	}
	return nil
}
