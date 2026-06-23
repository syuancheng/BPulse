package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

const (
	cloudBaseOpenIDHeader = "X-WX-OPENID"
	debugOpenIDHeader     = "X-Debug-OpenID"
	maxOpenIDLength       = 128
)

func Middleware(environment string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			openID, ok := trustedOpenID(r, environment)
			if !ok {
				writeUnauthenticated(w)
				return
			}
			identity := identityFromOpenID(openID)
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), identity)))
		})
	}
}

func trustedOpenID(r *http.Request, environment string) (string, bool) {
	debugOpenID := r.Header.Get(debugOpenIDHeader)
	if environment == "local" {
		return validOpenID(debugOpenID)
	}
	if debugOpenID != "" {
		return "", false
	}
	return validOpenID(r.Header.Get(cloudBaseOpenIDHeader))
}

func validOpenID(value string) (string, bool) {
	if value == "" || len(value) > maxOpenIDLength || strings.TrimSpace(value) != value {
		return "", false
	}
	return value, true
}

func writeUnauthenticated(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"code":    "UNAUTHENTICATED",
		"message": "authentication is required",
	})
}
