package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"skyward-api/internal/auth"
)

func TestSecurityHeadersApplied(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	tests := []struct {
		header string
		want   string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'"},
		{"Permissions-Policy", "camera=(), microphone=(), geolocation=()"},
	}

	for _, tc := range tests {
		got := rec.Header().Get(tc.header)
		if got != tc.want {
			t.Errorf("header %s = %q, want %q", tc.header, got, tc.want)
		}
	}
}

func TestAuthGuardRejectsRevokedSession(t *testing.T) {
	auth.ResetRevocationsForTest()
	secret := []byte("0123456789abcdef0123456789abcdef")
	userID := "test-user-revoked-session"

	token, err := auth.Sign(auth.Claims{Sub: userID, Username: "player1"}, secret)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	guarded := AuthGuard(secret, dummyHandler)

	// Token should pass initially
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("initial request should pass with 200, got %d", rec.Code)
	}

	// Invalidate user sessions
	auth.RevokeUserTokens(userID)

	// Old token should now be rejected with 401
	rec = httptest.NewRecorder()
	guarded.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("request with revoked token should get 401, got %d", rec.Code)
	}

	// New token should pass
	newToken, err := auth.Sign(auth.Claims{Sub: userID, Username: "player1"}, secret)
	if err != nil {
		t.Fatalf("Sign new: %v", err)
	}
	reqNew := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqNew.Header.Set("Authorization", "Bearer "+newToken)
	recNew := httptest.NewRecorder()
	guarded.ServeHTTP(recNew, reqNew)

	if recNew.Code != http.StatusOK {
		t.Fatalf("request with new token should pass with 200, got %d", recNew.Code)
	}
}
