package user

import (
	"context"
	"errors"
	"testing"

	"github.com/syuancheng/BPulse/server/internal/auth"
)

type fakeRepository struct {
	profile     Profile
	upsertCalls int
	updateCalls int
	lastPatch   PreferencesPatch
	err         error
}

func (r *fakeRepository) UpsertAndGet(context.Context, [32]byte) (Profile, error) {
	r.upsertCalls++
	return r.profile, r.err
}

func (r *fakeRepository) UpdatePreferences(_ context.Context, _ [32]byte, patch PreferencesPatch) (Profile, error) {
	r.updateCalls++
	r.lastPatch = patch
	return r.profile, r.err
}

func TestUpdatePreferencesValidation(t *testing.T) {
	validTimezone := "Asia/Singapore"
	invalidTimezone := "Mars/Olympus"
	localTimezone := "Local"
	tooLongTimezone := "Region/This-Timezone-Identifier-Is-Intentionally-Longer-Than-Sixty-Four-Characters"
	largeText := true
	tests := []struct {
		name    string
		patch   PreferencesPatch
		wantErr bool
	}{
		{name: "empty rejected", patch: PreferencesPatch{}, wantErr: true},
		{name: "invalid timezone rejected", patch: PreferencesPatch{Timezone: &invalidTimezone}, wantErr: true},
		{name: "local pseudo timezone rejected", patch: PreferencesPatch{Timezone: &localTimezone}, wantErr: true},
		{name: "oversized timezone rejected", patch: PreferencesPatch{Timezone: &tooLongTimezone}, wantErr: true},
		{name: "valid timezone", patch: PreferencesPatch{Timezone: &validTimezone}},
		{name: "valid accessibility", patch: PreferencesPatch{LargeTextEnabled: &largeText}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeRepository{}
			_, err := NewService(repository).UpdatePreferences(context.Background(), auth.Identity{}, test.patch)
			if (err != nil) != test.wantErr {
				t.Fatalf("UpdatePreferences() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantErr {
				if !errors.Is(err, ErrInvalidPreferences) {
					t.Fatalf("error = %v, want ErrInvalidPreferences", err)
				}
				if repository.upsertCalls != 0 || repository.updateCalls != 0 {
					t.Fatal("invalid patch reached repository")
				}
			} else if repository.upsertCalls != 1 || repository.updateCalls != 1 {
				t.Fatalf("repository calls = upsert %d update %d", repository.upsertCalls, repository.updateCalls)
			}
		})
	}
}

func TestGetOrCreateWrapsRepositoryError(t *testing.T) {
	repository := &fakeRepository{err: errors.New("synthetic database error")}
	if _, err := NewService(repository).GetOrCreate(context.Background(), auth.Identity{}); err == nil {
		t.Fatal("GetOrCreate() error = nil")
	}
}
