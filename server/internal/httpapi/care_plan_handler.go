package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/syuancheng/BPulse/server/internal/auth"
	"github.com/syuancheng/BPulse/server/internal/careplan"
	"github.com/syuancheng/BPulse/server/internal/user"
)

type CarePlanService interface {
	Create(ctx context.Context, input careplan.CreatePlanInput) (careplan.Plan, error)
	List(ctx context.Context, ownerID uint64) ([]careplan.Plan, error)
	Update(ctx context.Context, input careplan.UpdatePlanInput) (careplan.Plan, error)
	Delete(ctx context.Context, ownerID, planID uint64) error
}

type carePlanHandler struct {
	users     UserService
	carePlans CarePlanService
}

type carePlanRequest struct {
	TaskType  string `json:"taskType"`
	Title     string `json:"title"`
	LocalTime string `json:"localTime"`
	Enabled   *bool  `json:"enabled"`
}

type carePlanPatchRequest struct {
	Title     *string `json:"title"`
	LocalTime *string `json:"localTime"`
	Enabled   *bool   `json:"enabled"`
}

type carePlanResponse struct {
	ID         string `json:"id"`
	TaskType   string `json:"taskType"`
	Title      string `json:"title"`
	Recurrence string `json:"recurrence"`
	LocalTime  string `json:"localTime"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

func (h carePlanHandler) collection(w http.ResponseWriter, r *http.Request) {
	profile, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		plans, err := h.carePlans.List(r.Context(), profile.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"carePlans": newCarePlanResponses(plans)})
	case http.MethodPost:
		var request carePlanRequest
		if err := decodeStrictJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "request body is invalid")
			return
		}
		plan, err := h.carePlans.Create(r.Context(), careplan.CreatePlanInput{OwnerID: profile.ID, TaskType: request.TaskType, Title: request.Title, LocalTime: request.LocalTime, Enabled: request.Enabled})
		if errors.Is(err, careplan.ErrInvalid) {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "care plan is invalid")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
			return
		}
		writeJSON(w, http.StatusCreated, newCarePlanResponse(plan))
	default:
		methodNotAllowed(w, strings.Join([]string{http.MethodGet, http.MethodPost}, ", "))
	}
}

func (h carePlanHandler) item(w http.ResponseWriter, r *http.Request) {
	profile, ok := h.currentUser(w, r)
	if !ok {
		return
	}
	planID, ok := parseIDFromPath(w, r.URL.Path, "/api/v1/care-plans/")
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var request carePlanPatchRequest
		if err := decodeStrictJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "request body is invalid")
			return
		}
		plan, err := h.carePlans.Update(r.Context(), careplan.UpdatePlanInput{OwnerID: profile.ID, PlanID: planID, Title: request.Title, LocalTime: request.LocalTime, Enabled: request.Enabled})
		if errors.Is(err, careplan.ErrInvalid) {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "care plan is invalid")
			return
		}
		if errors.Is(err, careplan.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "care plan was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
			return
		}
		writeJSON(w, http.StatusOK, newCarePlanResponse(plan))
	case http.MethodDelete:
		err := h.carePlans.Delete(r.Context(), profile.ID, planID)
		if errors.Is(err, careplan.ErrNotFound) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "care plan was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, strings.Join([]string{http.MethodPatch, http.MethodDelete}, ", "))
	}
}

func (h carePlanHandler) currentUser(w http.ResponseWriter, r *http.Request) (user.Profile, bool) {
	identity, ok := auth.IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication is required")
		return user.Profile{}, false
	}
	profile, err := h.users.GetOrCreate(r.Context(), identity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
		return user.Profile{}, false
	}
	return profile, true
}

func parseIDFromPath(w http.ResponseWriter, path, prefix string) (uint64, bool) {
	value := strings.TrimPrefix(path, prefix)
	if value == "" || strings.Contains(value, "/") {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource was not found")
		return 0, false
	}
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "resource was not found")
		return 0, false
	}
	return id, true
}

func newCarePlanResponses(plans []careplan.Plan) []carePlanResponse {
	responses := make([]carePlanResponse, 0, len(plans))
	for _, plan := range plans {
		responses = append(responses, newCarePlanResponse(plan))
	}
	return responses
}

func newCarePlanResponse(plan careplan.Plan) carePlanResponse {
	return carePlanResponse{
		ID:         strconv.FormatUint(plan.ID, 10),
		TaskType:   plan.TaskType,
		Title:      plan.Title,
		Recurrence: plan.Recurrence,
		LocalTime:  plan.LocalTime,
		Enabled:    plan.Enabled,
		CreatedAt:  plan.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:  plan.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
