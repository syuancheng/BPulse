//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/syuancheng/BPulse/server/internal/config"
	"github.com/syuancheng/BPulse/server/internal/httpapi"
	"github.com/syuancheng/BPulse/server/internal/user"
)

type profilePayload struct {
	ID            string `json:"id"`
	Timezone      string `json:"timezone"`
	Accessibility struct {
		LargeText    bool `json:"largeText"`
		HighContrast bool `json:"highContrast"`
	} `json:"accessibility"`
}

func TestUserBootstrapPreferencesAndCrossUserIsolation(t *testing.T) {
	db := openDatabase(t)
	clearCareTaskTables(t, db)
	router := httpapi.NewRouter(httpapi.Dependencies{
		Environment: "local",
		Users:       user.NewService(user.NewMySQLRepository(db, 3*time.Second)),
	})

	patientA := getProfile(t, router, "synthetic-patient-a", "")
	if patientA.Timezone != "UTC" || patientA.ID == "" {
		t.Fatalf("initial patient A profile = %#v", patientA)
	}
	patientARepeat := getProfile(t, router, "synthetic-patient-a", "")
	if patientARepeat.ID != patientA.ID {
		t.Fatalf("repeated bootstrap IDs = %q and %q", patientA.ID, patientARepeat.ID)
	}

	patchRequest := httptest.NewRequest(http.MethodPatch, "/api/v1/me/preferences", bytes.NewBufferString(`{
  "timezone":"Asia/Singapore",
  "accessibility":{"largeText":true,"highContrast":true}
}`))
	patchRequest.Header.Set("X-Debug-OpenID", "synthetic-patient-a")
	patchRecorder := httptest.NewRecorder()
	router.ServeHTTP(patchRecorder, patchRequest)
	if patchRecorder.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body=%s", patchRecorder.Code, patchRecorder.Body.String())
	}

	patientB := getProfile(t, router, "synthetic-patient-b", "")
	if patientB.ID == patientA.ID {
		t.Fatal("different identities received the same internal user ID")
	}
	if patientB.Timezone != "UTC" || patientB.Accessibility.LargeText || patientB.Accessibility.HighContrast {
		t.Fatalf("patient B inherited patient A preferences: %#v", patientB)
	}

	patientAAfterPatch := getProfile(t, router, "synthetic-patient-a", "openid=synthetic-patient-b")
	if patientAAfterPatch.ID != patientA.ID || patientAAfterPatch.Timezone != "Asia/Singapore" || !patientAAfterPatch.Accessibility.LargeText || !patientAAfterPatch.Accessibility.HighContrast {
		t.Fatalf("patient A profile after patch = %#v", patientAAfterPatch)
	}

	var userCount int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM users").Scan(&userCount); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if userCount != 2 {
		t.Fatalf("user count = %d, want 2", userCount)
	}
	var invalidHashLengths int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM users WHERE OCTET_LENGTH(openid_hash) <> 32").Scan(&invalidHashLengths); err != nil {
		t.Fatalf("check identity hash lengths: %v", err)
	}
	if invalidHashLengths != 0 {
		t.Fatalf("users with invalid identity hash length = %d", invalidHashLengths)
	}
}

func TestProductionRejectsDebugIdentity(t *testing.T) {
	db := openDatabase(t)
	router := httpapi.NewRouter(httpapi.Dependencies{
		Environment: "production",
		Users:       user.NewService(user.NewMySQLRepository(db, 3*time.Second)),
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("X-Debug-OpenID", "synthetic-debug-user")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func getProfile(t *testing.T, router http.Handler, openID, rawQuery string) profilePayload {
	t.Helper()
	path := "/api/v1/me"
	if rawQuery != "" {
		path += "?" + rawQuery
	}
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("X-Debug-OpenID", openID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, body=%s", path, recorder.Code, recorder.Body.String())
	}
	var profile profilePayload
	if err := json.NewDecoder(recorder.Body).Decode(&profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	return profile
}

func openDatabase(t *testing.T) *sql.DB {
	t.Helper()
	if os.Getenv("MYSQL_DSN") == "" {
		t.Fatal("MYSQL_DSN is required for integration tests")
	}
	configuration, err := config.Load()
	if err != nil {
		t.Fatalf("load configuration: %v", err)
	}
	db, err := sql.Open("mysql", configuration.MySQLDSN)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Fatalf("ping database: %v", err)
	}
	var sessionTimezone string
	if err := db.QueryRowContext(ctx, "SELECT @@session.time_zone").Scan(&sessionTimezone); err != nil {
		db.Close()
		t.Fatalf("query session timezone: %v", err)
	}
	if sessionTimezone != "+00:00" {
		db.Close()
		t.Fatalf("session timezone = %q, want +00:00", sessionTimezone)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
