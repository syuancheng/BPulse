package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/syuancheng/BPulse/server/internal/auth"
	"github.com/syuancheng/BPulse/server/internal/user"
)

const maxPreferencesBodyBytes = 4096

type UserService interface {
	GetOrCreate(ctx context.Context, identity auth.Identity) (user.Profile, error)
	UpdatePreferences(ctx context.Context, identity auth.Identity, patch user.PreferencesPatch) (user.Profile, error)
}

type meHandler struct {
	users UserService
}

type profileResponse struct {
	ID            string                `json:"id"`
	Timezone      string                `json:"timezone"`
	Accessibility accessibilityResponse `json:"accessibility"`
	CreatedAt     string                `json:"createdAt"`
	UpdatedAt     string                `json:"updatedAt"`
}

type accessibilityResponse struct {
	LargeText    bool `json:"largeText"`
	HighContrast bool `json:"highContrast"`
}

type preferencesRequest struct {
	Timezone      *string                    `json:"timezone"`
	Accessibility *accessibilityPatchRequest `json:"accessibility"`
}

type accessibilityPatchRequest struct {
	LargeText    *bool `json:"largeText"`
	HighContrast *bool `json:"highContrast"`
}

func (h meHandler) get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
		return
	}
	profile, err := h.users.GetOrCreate(r.Context(), identity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
		return
	}
	writeJSON(w, http.StatusOK, newProfileResponse(profile))
}

func (h meHandler) patchPreferences(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		methodNotAllowed(w, http.MethodPatch)
		return
	}
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
		return
	}
	var request preferencesRequest
	if err := decodeStrictJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "request body is invalid")
		return
	}
	patch := user.PreferencesPatch{Timezone: request.Timezone}
	if request.Accessibility != nil {
		patch.LargeTextEnabled = request.Accessibility.LargeText
		patch.HighContrastEnabled = request.Accessibility.HighContrast
	}
	profile, err := h.users.UpdatePreferences(r.Context(), identity, patch)
	if errors.Is(err, user.ErrInvalidPreferences) {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "preferences are invalid")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
		return
	}
	writeJSON(w, http.StatusOK, newProfileResponse(profile))
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, destination interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxPreferencesBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func newProfileResponse(profile user.Profile) profileResponse {
	return profileResponse{
		ID:       strconv.FormatUint(profile.ID, 10),
		Timezone: profile.Timezone,
		Accessibility: accessibilityResponse{
			LargeText:    profile.LargeTextEnabled,
			HighContrast: profile.HighContrastEnabled,
		},
		CreatedAt: profile.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: profile.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
