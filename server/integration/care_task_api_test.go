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
	"strconv"
	"testing"
	"time"

	"github.com/syuancheng/BPulse/server/internal/careplan"
	"github.com/syuancheng/BPulse/server/internal/httpapi"
	"github.com/syuancheng/BPulse/server/internal/task"
	"github.com/syuancheng/BPulse/server/internal/user"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type carePlanListPayload struct {
	CarePlans []carePlanPayload `json:"carePlans"`
}

type carePlanPayload struct {
	ID        string `json:"id"`
	TaskType  string `json:"taskType"`
	Title     string `json:"title"`
	LocalTime string `json:"localTime"`
	Enabled   bool   `json:"enabled"`
}

type taskListPayload struct {
	Tasks []taskPayload `json:"tasks"`
}

type taskPayload struct {
	ID                 string  `json:"id"`
	CarePlanID         string  `json:"carePlanId"`
	TaskType           string  `json:"taskType"`
	ScheduledLocalDate string  `json:"scheduledLocalDate"`
	State              string  `json:"state"`
	CompletedAt        *string `json:"completedAt"`
	SkippedAt          *string `json:"skippedAt"`
}

func TestCarePlansAndTodayTasks(t *testing.T) {
	db := openDatabase(t)
	clearCareTaskTables(t, db)
	router := newCareTaskRouter(db, fixedClock{now: time.Date(2026, 6, 22, 17, 30, 0, 0, time.UTC)})

	patchPreferences(t, router, "synthetic-patient-a", `{"timezone":"Asia/Singapore"}`)

	bp := createCarePlan(t, router, "synthetic-patient-a", `{"taskType":"blood_pressure","title":"测血压","localTime":"07:00"}`)
	exercise := createCarePlan(t, router, "synthetic-patient-a", `{"taskType":"exercise","title":"散步","localTime":"18:30"}`)
	_ = createCarePlan(t, router, "synthetic-patient-a", `{"taskType":"diet","title":"记录晚餐","localTime":"19:30"}`)
	_ = createCarePlan(t, router, "synthetic-patient-a", `{"taskType":"medication_reminder_optional","title":"用药提醒","localTime":"08:00"}`)

	plans := listCarePlans(t, router, "synthetic-patient-a")
	if len(plans.CarePlans) != 4 {
		t.Fatalf("care plan count = %d, want 4", len(plans.CarePlans))
	}
	if len(listCarePlans(t, router, "synthetic-patient-b").CarePlans) != 0 {
		t.Fatal("patient B can see patient A care plans")
	}

	today := listTodayTasks(t, router, "synthetic-patient-a")
	if len(today.Tasks) != 4 {
		t.Fatalf("today task count = %d, want 4", len(today.Tasks))
	}
	if today.Tasks[0].ScheduledLocalDate != "2026-06-23" {
		t.Fatalf("scheduled local date = %s, want 2026-06-23", today.Tasks[0].ScheduledLocalDate)
	}
	_ = listTodayTasks(t, router, "synthetic-patient-a")
	var taskCount int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM task_instances").Scan(&taskCount); err != nil {
		t.Fatalf("count task instances: %v", err)
	}
	if taskCount != 4 {
		t.Fatalf("task count after duplicate generation = %d, want 4", taskCount)
	}

	completed := postTaskAction(t, router, "synthetic-patient-a", today.Tasks[0].ID, "complete", http.StatusOK)
	if completed.State != "completed" || completed.CompletedAt == nil {
		t.Fatalf("completed task = %#v", completed)
	}
	laterRouter := newCareTaskRouter(db, fixedClock{now: time.Date(2026, 6, 22, 18, 30, 0, 0, time.UTC)})
	completedAgain := postTaskAction(t, laterRouter, "synthetic-patient-a", today.Tasks[0].ID, "complete", http.StatusOK)
	if completedAgain.CompletedAt == nil || *completedAgain.CompletedAt != *completed.CompletedAt {
		t.Fatalf("repeated complete changed timestamp: first=%v again=%v", completed.CompletedAt, completedAgain.CompletedAt)
	}
	skipAfterComplete := postTaskAction(t, laterRouter, "synthetic-patient-a", today.Tasks[0].ID, "skip", http.StatusOK)
	if skipAfterComplete.State != "completed" || skipAfterComplete.SkippedAt != nil {
		t.Fatalf("skip after complete mutated terminal task: %#v", skipAfterComplete)
	}
	skipped := postTaskAction(t, router, "synthetic-patient-a", today.Tasks[1].ID, "skip", http.StatusOK)
	if skipped.State != "skipped" || skipped.SkippedAt == nil {
		t.Fatalf("skipped task = %#v", skipped)
	}
	postTaskAction(t, router, "synthetic-patient-b", today.Tasks[0].ID, "complete", http.StatusNotFound)

	patchCarePlan(t, router, "synthetic-patient-a", bp.ID, `{"localTime":"06:45"}`)
	patchCarePlan(t, router, "synthetic-patient-a", bp.ID, `{"localTime":"06:45"}`)
	_ = listTodayTasks(t, router, "synthetic-patient-a")
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM task_instances").Scan(&taskCount); err != nil {
		t.Fatalf("count task instances after plan time edit: %v", err)
	}
	if taskCount != 4 {
		t.Fatalf("task count after plan time edit = %d, want 4", taskCount)
	}
	patchCarePlan(t, router, "synthetic-patient-a", bp.ID, `{"enabled":false}`)
	deleteCarePlan(t, router, "synthetic-patient-a", exercise.ID)
	plans = listCarePlans(t, router, "synthetic-patient-a")
	if len(plans.CarePlans) != 3 {
		t.Fatalf("care plan count after delete = %d, want 3", len(plans.CarePlans))
	}
	for _, plan := range plans.CarePlans {
		if plan.ID == bp.ID && plan.Enabled {
			t.Fatal("patched care plan is still enabled")
		}
		if plan.ID == exercise.ID {
			t.Fatal("deleted care plan is still listed")
		}
	}
}

func TestCarePlanValidationAndOwnership(t *testing.T) {
	db := openDatabase(t)
	clearCareTaskTables(t, db)
	router := newCareTaskRouter(db, fixedClock{now: time.Date(2026, 6, 23, 1, 0, 0, 0, time.UTC)})

	request := newJSONRequest(http.MethodPost, "/api/v1/care-plans", "synthetic-patient-a", `{"taskType":"diagnosis","title":"诊断","localTime":"07:00"}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid create status = %d, want 400", recorder.Code)
	}

	plan := createCarePlan(t, router, "synthetic-patient-a", `{"taskType":"diet","title":"记录饮食","localTime":"12:00"}`)
	patch := newJSONRequest(http.MethodPatch, "/api/v1/care-plans/"+plan.ID, "synthetic-patient-b", `{"title":"越权修改"}`)
	patchRecorder := httptest.NewRecorder()
	router.ServeHTTP(patchRecorder, patch)
	if patchRecorder.Code != http.StatusNotFound {
		t.Fatalf("cross-user patch status = %d, want 404", patchRecorder.Code)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/api/v1/care-plans/"+plan.ID, nil)
	deleteRequest.Header.Set("X-Debug-OpenID", "synthetic-patient-b")
	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusNotFound {
		t.Fatalf("cross-user delete status = %d, want 404", deleteRecorder.Code)
	}
}

func newCareTaskRouter(db *sql.DB, clock task.Clock) http.Handler {
	userService := user.NewService(user.NewMySQLRepository(db, 3*time.Second))
	planRepository := careplan.NewMySQLRepository(db, 3*time.Second)
	return httpapi.NewRouter(httpapi.Dependencies{
		Environment: "local",
		Users:       userService,
		CarePlans:   careplan.NewService(planRepository),
		Tasks:       task.NewService(planRepository, task.NewMySQLRepository(db, 3*time.Second), clock),
	})
}

func clearCareTaskTables(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{"DELETE FROM bp_records", "DELETE FROM task_instances", "DELETE FROM care_plans", "DELETE FROM users"} {
		if _, err := db.ExecContext(context.Background(), statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

func patchPreferences(t *testing.T, router http.Handler, openID, body string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newJSONRequest(http.MethodPatch, "/api/v1/me/preferences", openID, body))
	if recorder.Code != http.StatusOK {
		t.Fatalf("patch preferences status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func createCarePlan(t *testing.T, router http.Handler, openID, body string) carePlanPayload {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newJSONRequest(http.MethodPost, "/api/v1/care-plans", openID, body))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create care plan status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload carePlanPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode care plan: %v", err)
	}
	if _, err := strconv.ParseUint(payload.ID, 10, 64); err != nil {
		t.Fatalf("care plan id %q is not numeric", payload.ID)
	}
	return payload
}

func listCarePlans(t *testing.T, router http.Handler, openID string) carePlanListPayload {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/care-plans", nil)
	request.Header.Set("X-Debug-OpenID", openID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list care plans status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload carePlanListPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode care plans: %v", err)
	}
	return payload
}

func patchCarePlan(t *testing.T, router http.Handler, openID, planID, body string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, newJSONRequest(http.MethodPatch, "/api/v1/care-plans/"+planID, openID, body))
	if recorder.Code != http.StatusOK {
		t.Fatalf("patch care plan status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func deleteCarePlan(t *testing.T, router http.Handler, openID, planID string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/care-plans/"+planID, nil)
	request.Header.Set("X-Debug-OpenID", openID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("delete care plan status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
}

func listTodayTasks(t *testing.T, router http.Handler, openID string) taskListPayload {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/today", nil)
	request.Header.Set("X-Debug-OpenID", openID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list tasks status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	var payload taskListPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode tasks: %v", err)
	}
	return payload
}

func postTaskAction(t *testing.T, router http.Handler, openID, taskID, action string, wantStatus int) taskPayload {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+taskID+"/"+action, nil)
	request.Header.Set("X-Debug-OpenID", openID)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if recorder.Code != wantStatus {
		t.Fatalf("task action status = %d, want %d, body=%s", recorder.Code, wantStatus, recorder.Body.String())
	}
	if wantStatus != http.StatusOK {
		return taskPayload{}
	}
	var payload taskPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode task action: %v", err)
	}
	return payload
}

func newJSONRequest(method, path, openID, body string) *http.Request {
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("X-Debug-OpenID", openID)
	request.Header.Set("Content-Type", "application/json")
	return request
}
