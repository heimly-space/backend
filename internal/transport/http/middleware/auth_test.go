package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	authinfra "heimly.space/backend/internal/infra/auth"
)

func TestJWTMiddlewareMissingToken(t *testing.T) {
	called := false
	handler := JWTMiddleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("next handler should not be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "missing token") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestJWTMiddlewareInvalidToken(t *testing.T) {
	called := false
	handler := JWTMiddleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if called {
		t.Fatal("next handler should not be called")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid token") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestJWTMiddlewareValidTokenSetsUserID(t *testing.T) {
	userID := uuid.New()
	token, err := authinfra.GenerateToken(userID, "secret", time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	handler := JWTMiddleware("secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID, ok := UserIDFromContext(r.Context())
		if !ok {
			t.Fatal("expected user id in context")
		}
		if gotID != userID {
			t.Fatalf("unexpected user id in context: got %s want %s", gotID, userID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
}
