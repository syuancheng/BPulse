package httpapi

import (
	"net/http"
	"strings"

	"github.com/syuancheng/BPulse/server/internal/bpentry"
)

type bpEntryHandler struct {
	users UserService
}

type interpretRequest struct {
	RecognizedText string `json:"recognizedText"`
}

func (h bpEntryHandler) interpret(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if _, ok := (carePlanHandler{users: h.users}).currentUser(w, r); !ok {
		return
	}
	var request interpretRequest
	if err := decodeStrictJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "request body is invalid")
		return
	}
	result, err := bpentry.InterpretText(strings.TrimSpace(request.RecognizedText))
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "recognized text is invalid")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
