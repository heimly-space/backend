package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"heimly.space/backend/internal/cfg"
	domain "heimly.space/backend/internal/domain/users"
	authinfra "heimly.space/backend/internal/infra/auth"
	usershttp "heimly.space/backend/internal/transport/http/users"
)

type routerRepoStub struct {
	createFn     func(ctx context.Context, login, email, name, hash string, birthday time.Time) (uuid.UUID, error)
	getByLoginFn func(ctx context.Context, login string) (*domain.UserWithPassword, error)
	getByIDFn    func(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

func (r *routerRepoStub) Create(
	ctx context.Context,
	login, email, name, hash string,
	birthday time.Time,
) (uuid.UUID, error) {
	if r.createFn == nil {
		return uuid.Nil, errors.New("unexpected Create call")
	}
	return r.createFn(ctx, login, email, name, hash, birthday)
}

func (r *routerRepoStub) GetByLogin(ctx context.Context, login string) (*domain.UserWithPassword, error) {
	if r.getByLoginFn == nil {
		return nil, errors.New("unexpected GetByLogin call")
	}
	return r.getByLoginFn(ctx, login)
}

func (r *routerRepoStub) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if r.getByIDFn == nil {
		return nil, errors.New("unexpected GetByID call")
	}
	return r.getByIDFn(ctx, id)
}

func newTestRouter(secret string, repo domain.Repository) nethttp.Handler {
	handlers := &usershttp.AuthHandlers{
		Users: &domain.Service{
			Repo:      repo,
			JWTSecret: secret,
			JWTExpiry: time.Hour,
		},
	}
	return NewRouter(handlers, &cfg.Config{JWTSecret: secret})
}

func TestRouterHealth(t *testing.T) {
	router := newTestRouter("secret", &routerRepoStub{})

	req := httptest.NewRequest(nethttp.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("unexpected health response: %+v", body)
	}
}

func TestRouterLoginRoute(t *testing.T) {
	userID := uuid.New()
	hash, err := authinfra.HashPassword("secret-pass")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	router := newTestRouter("secret", &routerRepoStub{
		getByLoginFn: func(_ context.Context, login string) (*domain.UserWithPassword, error) {
			if login != "john" {
				t.Fatalf("unexpected login: %s", login)
			}
			return &domain.UserWithPassword{
				User: domain.User{
					ID:    userID,
					Login: "john",
				},
				HashedPassword: hash,
			}, nil
		},
	})

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"login":"john","password":"secret-pass"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp usershttp.AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected token in response")
	}
	if _, err := authinfra.ParseToken(resp.Token, "secret"); err != nil {
		t.Fatalf("token should be valid: %v", err)
	}
}

func TestRouterUsersMeRouteProtected(t *testing.T) {
	router := newTestRouter("secret", &routerRepoStub{})

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/users/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "missing token") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestRouterUsersMeRouteAuthorized(t *testing.T) {
	userID := uuid.New()
	secret := "secret"
	router := newTestRouter(secret, &routerRepoStub{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.User, error) {
			if id != userID {
				t.Fatalf("unexpected id: %s", id)
			}
			return &domain.User{
				ID:       userID,
				Login:    "john",
				Email:    "john@example.com",
				Name:     "John Doe",
				Birthday: time.Date(1995, time.October, 15, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	})

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp usershttp.UserResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode profile response: %v", err)
	}
	if resp.ID != userID.String() {
		t.Fatalf("unexpected profile id: %s", resp.ID)
	}
}
