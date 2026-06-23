package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/syuancheng/BPulse/server/internal/auth"
	"github.com/syuancheng/BPulse/server/internal/user"
)

type fakeUserService struct {
	profile          user.Profile
	getIdentities    []auth.Identity
	updateIdentities []auth.Identity
	lastPatch        user.PreferencesPatch
	err              error
}

func (s *fakeUserService) GetOrCreate(_ context.Context, identity auth.Identity) (user.Profile, error) {
	s.getIdentities = append(s.getIdentities, identity)
	return s.profile, s.err
}

func (s *fakeUserService) UpdatePreferences(_ context.Context, identity auth.Identity, patch user.PreferencesPatch) (user.Profile, error) {
	s.updateIdentities = append(s.updateIdentities, identity)
	s.lastPatch = patch
	return s.profile, s.err
}

func TestGetMeUsesOnlyAuthenticatedHeaderIdentity(t *testing.T) {
	service := &fakeUserService{profile: syntheticProfile()}
	router := NewRouter(Dependencies{Environment: "local", Users: service})

	requestWithQuery := httptest.NewRequest(http.MethodGet, "/api/v1/me?openid=synthetic-user-b", nil)
	requestWithQuery.Header.Set("X-Debug-OpenID", "synthetic-user-a")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, requestWithQuery)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	plainRequest := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	plainRequest.Header.Set("X-Debug-OpenID", "synthetic-user-a")
	router.ServeHTTP(httptest.NewRecorder(), plainRequest)

	if len(service.getIdentities) != 2 || service.getIdentities[0] != service.getIdentities[1] {
		t.Fatal("query openid changed authenticated identity")
	}
	if strings.Contains(recorder.Body.String(), "synthetic-user-a") || strings.Contains(recorder.Body.String(), "synthetic-user-b") {
		t.Fatal("response leaked plaintext identity")
	}
}

func TestPatchPreferencesUsesStrictJSON(t *testing.T) {
	service := &fakeUserService{profile: syntheticProfile()}
	router := NewRouter(Dependencies{Environment: "local", Users: service})

	valid := httptest.NewRequest(http.MethodPatch, "/api/v1/me/preferences", bytes.NewBufferString(`{
  "timezone":"Asia/Singapore",
  "accessibility":{"largeText":true,"highContrast":false}
}`))
	valid.Header.Set("X-Debug-OpenID", "synthetic-user-a")
	validRecorder := httptest.NewRecorder()
	router.ServeHTTP(validRecorder, valid)
	if validRecorder.Code != http.StatusOK {
		t.Fatalf("valid status = %d, body=%s", validRecorder.Code, validRecorder.Body.String())
	}
	if service.lastPatch.Timezone == nil || *service.lastPatch.Timezone != "Asia/Singapore" {
		t.Fatalf("timezone patch = %#v", service.lastPatch.Timezone)
	}
	if service.lastPatch.LargeTextEnabled == nil || !*service.lastPatch.LargeTextEnabled {
		t.Fatal("largeText patch was not passed to service")
	}

	unknownIdentity := httptest.NewRequest(http.MethodPatch, "/api/v1/me/preferences", bytes.NewBufferString(`{"openid":"synthetic-user-b"}`))
	unknownIdentity.Header.Set("X-Debug-OpenID", "synthetic-user-a")
	unknownRecorder := httptest.NewRecorder()
	router.ServeHTTP(unknownRecorder, unknownIdentity)
	if unknownRecorder.Code != http.StatusBadRequest {
		t.Fatalf("unknown identity status = %d, want 400", unknownRecorder.Code)
	}
	if len(service.updateIdentities) != 1 {
		t.Fatal("body openid reached user service")
	}
}

func TestNonLocalDebugIdentityIsRejected(t *testing.T) {
	service := &fakeUserService{profile: syntheticProfile()}
	router := NewRouter(Dependencies{Environment: "test", Users: service})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("X-Debug-OpenID", "synthetic-debug-user")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if len(service.getIdentities) != 0 {
		t.Fatal("rejected debug identity reached user service")
	}
}

func TestInternalErrorDoesNotLeakIdentityOrCause(t *testing.T) {
	service := &fakeUserService{err: errors.New("database failed for synthetic-secret-openid")}
	router := NewRouter(Dependencies{Environment: "local", Users: service})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("X-Debug-OpenID", "synthetic-secret-openid")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "synthetic-secret-openid") || strings.Contains(recorder.Body.String(), "database failed") {
		t.Fatalf("error response leaked details: %s", recorder.Body.String())
	}
}

func syntheticProfile() user.Profile {
	return user.Profile{
		ID:        9007199254740993,
		Timezone:  "UTC",
		CreatedAt: time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC),
	}
}
