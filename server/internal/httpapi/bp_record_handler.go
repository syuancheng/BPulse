package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/syuancheng/BPulse/server/internal/bprecord"
)

type BPRecordService interface {
	Create(ctx context.Context, input bprecord.CreateInput) (bprecord.Record, error)
	List(ctx context.Context, filter bprecord.ListFilter) ([]bprecord.Record, error)
	Get(ctx context.Context, ownerID, recordID uint64) (bprecord.Record, error)
	Trends(ctx context.Context, ownerID uint64, timezone string, days int) ([]bprecord.TrendPoint, error)
}

type bpRecordHandler struct {
	users     UserService
	bpRecords BPRecordService
}

type bpRecordRequest struct {
	ClientRequestID string                  `json:"clientRequestId"`
	MeasuredAt      string                  `json:"measuredAt"`
	Timezone        string                  `json:"timezone"`
	EntryMethod     string                  `json:"entryMethod"`
	Readings        []bprecord.ReadingInput `json:"readings"`
	Note            *string                 `json:"note"`
	TimeConfirmed   bool                    `json:"timeConfirmed"`
}

type bpRecordResponse struct {
	ID              string                    `json:"id"`
	ClientRequestID string                    `json:"clientRequestId"`
	MeasuredAt      string                    `json:"measuredAt"`
	Timezone        string                    `json:"timezone"`
	EntryMethod     string                    `json:"entryMethod"`
	Readings        []bprecord.ReadingPayload `json:"readings"`
	Summary         bprecord.SummaryPayload   `json:"summary"`
	Note            *string                   `json:"note,omitempty"`
	CreatedAt       string                    `json:"createdAt"`
}

type trendPointResponse struct {
	LocalDate        string `json:"localDate"`
	AverageSystolic  int    `json:"averageSystolic"`
	AverageDiastolic int    `json:"averageDiastolic"`
	AveragePulse     *int   `json:"averagePulse,omitempty"`
	RecordCount      int    `json:"recordCount"`
}

func (h bpRecordHandler) collection(w http.ResponseWriter, r *http.Request) {
	profile, ok := (carePlanHandler{users: h.users}).currentUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodPost:
		var request bpRecordRequest
		if err := decodeStrictJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "request body is invalid")
			return
		}
		record, err := h.bpRecords.Create(r.Context(), bprecord.CreateInput{
			OwnerID:         profile.ID,
			ClientRequestID: request.ClientRequestID,
			MeasuredAt:      request.MeasuredAt,
			Timezone:        request.Timezone,
			EntryMethod:     request.EntryMethod,
			Readings:        request.Readings,
			Note:            request.Note,
			TimeConfirmed:   request.TimeConfirmed,
		})
		if errors.Is(err, bprecord.ErrInvalid) {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "blood pressure record is invalid")
			return
		}
		if errors.Is(err, bprecord.ErrConflict) {
			writeError(w, http.StatusConflict, "CONFLICT", "client request id was already used for different data")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
			return
		}
		writeJSON(w, http.StatusCreated, newBPRecordResponse(record))
	case http.MethodGet:
		filter, ok := parseBPRecordListFilter(w, r, profile.ID)
		if !ok {
			return
		}
		records, err := h.bpRecords.List(r.Context(), filter)
		if errors.Is(err, bprecord.ErrInvalid) {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "record query is invalid")
			return
		}
		if errors.Is(err, bprecord.ErrDecryptFailed) {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
			return
		}
		responses := make([]bpRecordResponse, 0, len(records))
		for _, record := range records {
			responses = append(responses, newBPRecordResponse(record))
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"records": responses})
	default:
		methodNotAllowed(w, strings.Join([]string{http.MethodGet, http.MethodPost}, ", "))
	}
}

func (h bpRecordHandler) item(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	profile, ok := (carePlanHandler{users: h.users}).currentUser(w, r)
	if !ok {
		return
	}
	recordID, ok := parseIDFromPath(w, r.URL.Path, "/api/v1/bp-records/")
	if !ok {
		return
	}
	record, err := h.bpRecords.Get(r.Context(), profile.ID, recordID)
	if errors.Is(err, bprecord.ErrNotFound) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "blood pressure record was not found")
		return
	}
	if errors.Is(err, bprecord.ErrDecryptFailed) {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
		return
	}
	writeJSON(w, http.StatusOK, newBPRecordResponse(record))
}

func (h bpRecordHandler) trends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	profile, ok := (carePlanHandler{users: h.users}).currentUser(w, r)
	if !ok {
		return
	}
	days := 7
	if value := r.URL.Query().Get("days"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "days is invalid")
			return
		}
		days = parsed
	}
	points, err := h.bpRecords.Trends(r.Context(), profile.ID, profile.Timezone, days)
	if errors.Is(err, bprecord.ErrInvalid) {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "trend query is invalid")
		return
	}
	if errors.Is(err, bprecord.ErrDecryptFailed) {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "request could not be completed")
		return
	}
	responses := make([]trendPointResponse, 0, len(points))
	for _, point := range points {
		responses = append(responses, trendPointResponse(point))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"points": responses})
}

func parseBPRecordListFilter(w http.ResponseWriter, r *http.Request, ownerID uint64) (bprecord.ListFilter, bool) {
	filter := bprecord.ListFilter{OwnerID: ownerID}
	query := r.URL.Query()
	if value := query.Get("from"); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "from is invalid")
			return bprecord.ListFilter{}, false
		}
		utc := parsed.UTC()
		filter.FromUTC = &utc
	}
	if value := query.Get("to"); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "to is invalid")
			return bprecord.ListFilter{}, false
		}
		utc := parsed.UTC()
		filter.ToUTC = &utc
	}
	if filter.FromUTC != nil && filter.ToUTC != nil && !filter.FromUTC.Before(*filter.ToUTC) {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "from must be before to")
		return bprecord.ListFilter{}, false
	}
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "limit is invalid")
			return bprecord.ListFilter{}, false
		}
		filter.Limit = parsed
	}
	return filter, true
}

func newBPRecordResponse(record bprecord.Record) bpRecordResponse {
	return bpRecordResponse{
		ID:              strconv.FormatUint(record.ID, 10),
		ClientRequestID: record.ClientRequestID,
		MeasuredAt:      record.MeasuredAtUTC.UTC().Format(time.RFC3339Nano),
		Timezone:        record.Timezone,
		EntryMethod:     record.EntryMethod,
		Readings:        record.Payload.Readings,
		Summary:         record.Payload.Summary,
		Note:            record.Payload.Note,
		CreatedAt:       record.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}
