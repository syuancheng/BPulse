package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareIdentityMatrix(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		cloudOpenID string
		debugOpenID string
		wantStatus  int
	}{
		{name: "local debug accepted", environment: "local", debugOpenID: "synthetic-local-user", wantStatus: http.StatusNoContent},
		{name: "local cloud header ignored", environment: "local", cloudOpenID: "synthetic-cloud-user", wantStatus: http.StatusUnauthorized},
		{name: "test cloud accepted", environment: "test", cloudOpenID: "synthetic-test-user", wantStatus: http.StatusNoContent},
		{name: "test debug rejected", environment: "test", debugOpenID: "synthetic-debug-user", wantStatus: http.StatusUnauthorized},
		{name: "test debug rejected even with cloud", environment: "test", cloudOpenID: "synthetic-test-user", debugOpenID: "synthetic-debug-user", wantStatus: http.StatusUnauthorized},
		{name: "production cloud accepted", environment: "production", cloudOpenID: "synthetic-production-user", wantStatus: http.StatusNoContent},
		{name: "production debug rejected", environment: "production", debugOpenID: "synthetic-debug-user", wantStatus: http.StatusUnauthorized},
		{name: "missing identity rejected", environment: "production", wantStatus: http.StatusUnauthorized},
		{name: "surrounding whitespace rejected", environment: "local", debugOpenID: " synthetic-user ", wantStatus: http.StatusUnauthorized},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, ok := IdentityFromContext(r.Context()); !ok {
					t.Fatal("authenticated request has no identity in context")
				}
				w.WriteHeader(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			request.Header.Set(cloudBaseOpenIDHeader, test.cloudOpenID)
			request.Header.Set(debugOpenIDHeader, test.debugOpenID)
			recorder := httptest.NewRecorder()

			Middleware(test.environment)(next).ServeHTTP(recorder, request)
			if recorder.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, test.wantStatus)
			}
		})
	}
}

func TestMiddlewareStoresOnlyHashedIdentity(t *testing.T) {
	const openID = "synthetic-sensitive-identity"
	var got Identity
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = IdentityFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set(debugOpenIDHeader, openID)
	recorder := httptest.NewRecorder()

	Middleware("local")(next).ServeHTTP(recorder, request)

	if string(got.Reference[:]) == openID {
		t.Fatal("identity context contains plaintext openid")
	}
}
