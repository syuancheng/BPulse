//go:build integration
// +build integration

package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/syuancheng/BPulse/server/internal/bprecord"
	"github.com/syuancheng/BPulse/server/internal/httpapi"
	"github.com/syuancheng/BPulse/server/internal/user"
)

type bpRecordPayload struct {
	ID              string `json:"id"`
	ClientRequestID string `json:"clientRequestId"`
	MeasuredAt      string `json:"measuredAt"`
	Timezone        string `json:"timezone"`
	EntryMethod     string `json:"entryMethod"`
	Readings        []struct {
		Systolic  int  `json:"systolic"`
		Diastolic int  `json:"diastolic"`
		Pulse     *int `json:"pulse"`
	} `json:"readings"`
	Summary struct {
		AverageSystolic  int  `json:"averageSystolic"`
		AverageDiastolic int  `json:"averageDiastolic"`
		AveragePulse     *int `json:"averagePulse"`
		ReadingCount     int  `json:"readingCount"`
	} `json:"summary"`
}

type bpRecordListPayload struct {
	Records []bpRecordPayload `json:"records"`
}

type bpTrendPayload struct {
	Points []struct {
		LocalDate        string `json:"localDate"`
		AverageSystolic  int    `json:"averageSystolic"`
		AverageDiastolic int    `json:"averageDiastolic"`
		RecordCount      int    `json:"recordCount"`
	} `json:"points"`
}

func TestBPRecordCreateListGetTrendAndEncryption(t *testing.T) {
	db := openDatabase(t)
	clearCareTaskTables(t, db)
	router := newBPRecordRouter(t, db, fixedClock{now: time.Date(2026, 6, 23, 11, 0, 0, 0, time.UTC)})
	patchPreferences(t, router, "synthetic-patient-a", `{"timezone":"Asia/Singapore"}`)

	body := `{
  "clientRequestId":"req-bp-12345678",
  "measuredAt":"2026-06-23T18:30:00+08:00",
  "timezone":"Asia/Singapore",
  "entryMethod":"manual",
  "readings":[
    {"systolic":131,"diastolic":83,"pulse":70},
    {"systolic":133,"diastolic":85,"pulse":72}
  ],
  "note":"synthetic note"
}`
	created := postBPRecord(t, router, "synthetic-patient-a", body, http.StatusCreated)
	if created.MeasuredAt != "2026-06-23T10:30:00Z" || created.Summary.AverageSystolic != 132 || created.Summary.AverageDiastolic != 84 || *created.Summary.AveragePulse != 71 {
		t.Fatalf("created record = %#v", created)
	}
	createdAgain := postBPRecord(t, router, "synthetic-patient-a", body, http.StatusCreated)
	if createdAgain.ID != created.ID {
		t.Fatalf("repeated save returned id %s, want %s", createdAgain.ID, created.ID)
	}
	var count int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM bp_records").Scan(&count); err != nil {
		t.Fatalf("count bp records: %v", err)
	}
	if count != 1 {
		t.Fatalf("bp record count = %d, want 1", count)
	}
	var ciphertext []byte
	if err := db.QueryRowContext(context.Background(), "SELECT ciphertext FROM bp_records WHERE id = ?", created.ID).Scan(&ciphertext); err != nil {
		t.Fatalf("read ciphertext: %v", err)
	}
	if strings.Contains(string(ciphertext), "synthetic note") || strings.Contains(string(ciphertext), "132") {
		t.Fatal("ciphertext contains plaintext health payload")
	}

	list := listBPRecords(t, router, "synthetic-patient-a")
	if len(list.Records) != 1 || list.Records[0].ID != created.ID {
		t.Fatalf("list = %#v", list)
	}
	if len(listBPRecords(t, router, "synthetic-patient-b").Records) != 0 {
		t.Fatal("patient B can list patient A records")
	}
	getBPRecord(t, router, "synthetic-patient-b", created.ID, http.StatusNotFound)
	got := getBPRecord(t, router, "synthetic-patient-a", created.ID, http.StatusOK)
	if got.ID != created.ID {
		t.Fatalf("get id = %s, want %s", got.ID, created.ID)
	}

	trends := getBPTrends(t, router, "synthetic-patient-a")
	if len(trends.Points) != 1 || trends.Points[0].LocalDate != "2026-06-23" || trends.Points[0].AverageSystolic != 132 {
		t.Fatalf("trends = %#v", trends)
	}
}

func TestBPRecordValidationAndCorruptedCiphertext(t *testing.T) {
	db := openDatabase(t)
	clearCareTaskTables(t, db)
	router := newBPRecordRouter(t, db, fixedClock{now: time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)})

	postBPRecord(t, router, "synthetic-patient-a", `{
  "clientRequestId":"req-bp-invalid",
  "measuredAt":"2026-06-23T18:30:00+08:00",
  "timezone":"Asia/Singapore",
  "entryMethod":"manual",
  "readings":[{"systolic":120,"diastolic":200}]
}`, http.StatusBadRequest)
	postBPRecord(t, router, "synthetic-patient-a", `{
  "clientRequestId":"req-bp-oldtime",
  "measuredAt":"2026-05-01T10:00:00Z",
  "timezone":"UTC",
  "entryMethod":"manual",
  "readings":[{"systolic":120,"diastolic":80}]
}`, http.StatusBadRequest)
	created := postBPRecord(t, router, "synthetic-patient-a", `{
  "clientRequestId":"req-bp-oldtime-ok",
  "measuredAt":"2026-05-01T10:00:00Z",
  "timezone":"UTC",
  "entryMethod":"manual",
  "timeConfirmed":true,
  "readings":[{"systolic":120,"diastolic":80}]
}`, http.StatusCreated)
	if _, err := db.ExecContext(context.Background(), "UPDATE bp_records SET ciphertext = X'000102' WHERE id = ?", created.ID); err != nil {
		t.Fatalf("corrupt ciphertext: %v", err)
	}
	getBPRecord(t, router, "synthetic-patient-a", created.ID, http.StatusInternalServerError)
}

func newBPRecordRouter(t *testing.T, db *sql.DB, clock bprecord.Clock) http.Handler {
	t.Helper()
	keyring, err := bprecord.NewKeyringFromBase64("test-v1", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatalf("NewKeyringFromBase64() error = %v", err)
	}
	userService := user.NewService(user.NewMySQLRepository(db, 3*time.Second))
	return httpapi.NewRouter(httpapi.Dependencies{
		Environment: "local",
		Users:       userService,
		BPRecords:   bprecord.NewService(bprecord.NewMySQLRepository(db, 3*time.Second), keyring, clock),
	})
}

func postBPRecord(t *testing.T, router http.Handler, openID, body string, wantStatus int) bpRecordPayload {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newJSONRequest(http.MethodPost, "/api/v1/bp-records", openID, body))
	if recorder.Code != wantStatus {
		t.Fatalf("post bp record status = %d, want %d, body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	if wantStatus != http.StatusCreated {
		return bpRecordPayload{}
	}
	var payload bpRecordPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode bp record: %v", err)
	}
	return payload
}

func listBPRecords(t *testing.T, router http.Handler, openID string) bpRecordListPayload {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/bp-records", nil)
	request.Header.Set("X-Debug-OpenID", openID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list bp records status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload bpRecordListPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode bp record list: %v", err)
	}
	return payload
}

func getBPRecord(t *testing.T, router http.Handler, openID, id string, wantStatus int) bpRecordPayload {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/bp-records/"+id, nil)
	request.Header.Set("X-Debug-OpenID", openID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != wantStatus {
		t.Fatalf("get bp record status = %d, want %d, body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	if wantStatus != http.StatusOK {
		return bpRecordPayload{}
	}
	var payload bpRecordPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode bp record get: %v", err)
	}
	return payload
}

func getBPTrends(t *testing.T, router http.Handler, openID string) bpTrendPayload {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/bp-trends?days=7", nil)
	request.Header.Set("X-Debug-OpenID", openID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("get bp trends status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload bpTrendPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode bp trends: %v", err)
	}
	return payload
}
