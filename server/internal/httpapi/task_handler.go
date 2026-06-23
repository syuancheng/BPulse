package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/syuancheng/BPulse/server/internal/task"
)

type TaskService interface {
	Today(ctx context.Context, ownerID uint64, timezone string) ([]task.Instance, error)
	Complete(ctx context.Context, ownerID, taskID uint64) (task.Instance, error)
	Skip(ctx context.Context, ownerID, taskID uint64) (task.Instance, error)
}

type taskHandler struct {
	users UserService
	tasks TaskService
}

type taskResponse struct {
	ID                 string  `json:"id"`
	CarePlanID         string  `json:"carePlanId"`
	TaskType           string  `json:"taskType"`
	Title              string  `json:"title"`
	ScheduledLocalDate string  `json:"scheduledLocalDate"`
	ScheduledAt        string  `json:"scheduledAt"`
	State              string  `json:"state"`
	CompletedAt        *string `json:"completedAt,omitempty"`
	SkippedAt          *string `json:"skippedAt,omitempty"`
}

func (h taskHandler) today(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	profile, ok := (carePlanHandler{users: h.users}).currentUser(w, r)
	if !ok {
		return
	}
	tasks, err := h.tasks.Today(r.Context(), profile.ID, profile.Timezone)
	if errors.Is(err, task.ErrInvalid) {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "task request is invalid")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"tasks": newTaskResponses(tasks)})
}

func (h taskHandler) action(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	profile, ok := (carePlanHandler{users: h.users}).currentUser(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "task was not found")
		return
	}
	taskID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || taskID == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "task was not found")
		return
	}
	var instance task.Instance
	switch parts[1] {
	case "complete":
		instance, err = h.tasks.Complete(r.Context(), profile.ID, taskID)
	case "skip":
		instance, err = h.tasks.Skip(r.Context(), profile.ID, taskID)
	default:
		writeError(w, http.StatusNotFound, "NOT_FOUND", "task was not found")
		return
	}
	if errors.Is(err, task.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "task was not found")
		return
	}
	if errors.Is(err, task.ErrInvalid) {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "task request is invalid")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
		return
	}
	writeJSON(w, http.StatusOK, newTaskResponse(instance))
}

func newTaskResponses(tasks []task.Instance) []taskResponse {
	responses := make([]taskResponse, 0, len(tasks))
	for _, instance := range tasks {
		responses = append(responses, newTaskResponse(instance))
	}
	return responses
}

func newTaskResponse(instance task.Instance) taskResponse {
	response := taskResponse{
		ID:                 strconv.FormatUint(instance.ID, 10),
		CarePlanID:         strconv.FormatUint(instance.CarePlanID, 10),
		TaskType:           instance.TaskType,
		Title:              instance.Title,
		ScheduledLocalDate: instance.ScheduledLocalDate,
		ScheduledAt:        instance.ScheduledAtUTC.UTC().Format(time.RFC3339Nano),
		State:              instance.State,
	}
	if instance.CompletedAtUTC != nil {
		value := instance.CompletedAtUTC.UTC().Format(time.RFC3339Nano)
		response.CompletedAt = &value
	}
	if instance.SkippedAtUTC != nil {
		value := instance.SkippedAtUTC.UTC().Format(time.RFC3339Nano)
		response.SkippedAt = &value
	}
	return response
}
