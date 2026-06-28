package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/syuancheng/BPulse/server/internal/auth"
)

type Dependencies struct {
	Environment string
	Users       UserService
	CarePlans   CarePlanService
	Tasks       TaskService
	BPRecords   BPRecordService
}

func NewRouter(dependencies Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	protected := auth.Middleware(dependencies.Environment)
	me := meHandler{users: dependencies.Users}
	carePlans := carePlanHandler{users: dependencies.Users, carePlans: dependencies.CarePlans}
	tasks := taskHandler{users: dependencies.Users, tasks: dependencies.Tasks}
	bpRecords := bpRecordHandler{users: dependencies.Users, bpRecords: dependencies.BPRecords}
	bpEntries := bpEntryHandler{users: dependencies.Users}
	mux.Handle("/api/v1/me", protected(http.HandlerFunc(me.get)))
	mux.Handle("/api/v1/me/preferences", protected(http.HandlerFunc(me.patchPreferences)))
	mux.Handle("/api/v1/care-plans", protected(http.HandlerFunc(carePlans.collection)))
	mux.Handle("/api/v1/care-plans/", protected(http.HandlerFunc(carePlans.item)))
	mux.Handle("/api/v1/tasks/today", protected(http.HandlerFunc(tasks.today)))
	mux.Handle("/api/v1/tasks/", protected(http.HandlerFunc(tasks.action)))
	mux.Handle("/api/v1/bp-records", protected(http.HandlerFunc(bpRecords.collection)))
	mux.Handle("/api/v1/bp-records/", protected(http.HandlerFunc(bpRecords.item)))
	mux.Handle("/api/v1/bp-trends", protected(http.HandlerFunc(bpRecords.trends)))
	mux.Handle("/api/v1/bp-entry/interpret", protected(http.HandlerFunc(bpEntries.interpret)))
	return mux
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "INVALID_ARGUMENT", "method is not allowed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
