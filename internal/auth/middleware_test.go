package auth_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/benakaben10/sns/internal/auth"
)

const testSecret = "test-secret-key-for-unit-tests"

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func makeToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}

func newVerifier() auth.Verifier {
	return auth.NewHMACVerifierForTest(testSecret)
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func applyMiddleware(verifier auth.Verifier) http.Handler {
	return auth.Middleware(verifier, newTestLogger())(http.HandlerFunc(okHandler))
}

func TestMiddleware_MissingToken(t *testing.T) {
	h := applyMiddleware(newVerifier())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestMiddleware_InvalidBearerFormat(t *testing.T) {
	h := applyMiddleware(newVerifier())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestMiddleware_BearerWithEmptyToken(t *testing.T) {
	h := applyMiddleware(newVerifier())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestMiddleware_ExpiredToken(t *testing.T) {
	h := applyMiddleware(newVerifier())
	token := makeToken(t, jwt.MapClaims{
		"sub": "user1",
		"exp": time.Now().Add(-1 * time.Hour).Unix(),
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestMiddleware_InvalidSignature(t *testing.T) {
	h := applyMiddleware(newVerifier())
	// Token signed with wrong secret
	wrongToken := makeToken(t, jwt.MapClaims{
		"sub": "user1",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	// Tamper by replacing last char
	wrongToken = wrongToken[:len(wrongToken)-1] + "X"
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+wrongToken)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestMiddleware_ValidToken(t *testing.T) {
	h := applyMiddleware(newVerifier())
	token := makeToken(t, jwt.MapClaims{
		"sub":   "user1",
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"scope": "email:send",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRequireAnyPermission_MissingScope_Returns403(t *testing.T) {
	verifier := newVerifier()
	inner := auth.RequireAnyPermission(
		func(c *auth.Claims) bool { return c.HasScope("email:send") },
		func(c *auth.Claims) bool { return c.HasRealmRole("email_sender") },
		func(c *auth.Claims) bool { return c.HasRealmRole("admin") },
	)(http.HandlerFunc(okHandler))
	h := auth.Middleware(verifier, newTestLogger())(inner)

	token := makeToken(t, jwt.MapClaims{
		"sub":   "user1",
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
		"scope": "read:profile",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestRequireAnyPermission_WithEmailSendScope_Returns200(t *testing.T) {
	verifier := newVerifier()
	inner := auth.RequireAnyPermission(
		func(c *auth.Claims) bool { return c.HasScope("email:send") },
		func(c *auth.Claims) bool { return c.HasRealmRole("email_sender") },
		func(c *auth.Claims) bool { return c.HasRealmRole("admin") },
	)(http.HandlerFunc(okHandler))
	h := auth.Middleware(verifier, newTestLogger())(inner)

	token := makeToken(t, jwt.MapClaims{
		"sub":   "user1",
		"exp":   time.Now().Add(1 * time.Hour).Unix(),
		"scope": "email:send other:scope",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireAnyPermission_WithEmailSenderRole_Returns200(t *testing.T) {
	verifier := newVerifier()
	inner := auth.RequireAnyPermission(
		func(c *auth.Claims) bool { return c.HasScope("email:send") },
		func(c *auth.Claims) bool { return c.HasRealmRole("email_sender") },
		func(c *auth.Claims) bool { return c.HasRealmRole("admin") },
	)(http.HandlerFunc(okHandler))
	h := auth.Middleware(verifier, newTestLogger())(inner)

	token := makeToken(t, jwt.MapClaims{
		"sub": "user1",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"realm_access": map[string]interface{}{
			"roles": []string{"email_sender", "user"},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRequireAnyPermission_WithAdminRole_Returns200(t *testing.T) {
	verifier := newVerifier()
	inner := auth.RequireAnyPermission(
		func(c *auth.Claims) bool { return c.HasScope("email:send") },
		func(c *auth.Claims) bool { return c.HasRealmRole("email_sender") },
		func(c *auth.Claims) bool { return c.HasRealmRole("admin") },
	)(http.HandlerFunc(okHandler))
	h := auth.Middleware(verifier, newTestLogger())(inner)

	token := makeToken(t, jwt.MapClaims{
		"sub": "user1",
		"exp": time.Now().Add(1 * time.Hour).Unix(),
		"realm_access": map[string]interface{}{
			"roles": []string{"admin"},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
