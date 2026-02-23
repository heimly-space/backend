package users

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	domain "heimly.space/backend/internal/domain/users"
	"heimly.space/backend/internal/httpdto"
	authinfra "heimly.space/backend/internal/infra/auth"
	httpmw "heimly.space/backend/internal/transport/http/middleware"
)

type handlersRepoStub struct {
	createFn     func(ctx context.Context, login, email, name, hash string, birthday time.Time) (uuid.UUID, error)
	getByLoginFn func(ctx context.Context, login string) (*domain.UserWithPassword, error)
	getByIDFn    func(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

func (r *handlersRepoStub) Create(
	ctx context.Context,
	login, email, name, hash string,
	birthday time.Time,
) (uuid.UUID, error) {
	return r.createFn(ctx, login, email, name, hash, birthday)
}

func (r *handlersRepoStub) GetByLogin(ctx context.Context, login string) (*domain.UserWithPassword, error) {
	return r.getByLoginFn(ctx, login)
}

func (r *handlersRepoStub) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	return r.getByIDFn(ctx, id)
}

func newAuthHandlers(secret string, repo domain.Repository) *AuthHandlers {
	return &AuthHandlers{
		Users: &domain.Service{
			Repo:      repo,
			JWTSecret: secret,
			JWTExpiry: time.Hour,
		},
	}
}

func TestRegisterHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	expectedBirthday := time.Date(1995, time.October, 15, 0, 0, 0, 0, time.UTC)
	repo := &handlersRepoStub{
		createFn: func(_ context.Context, login, email, name, hash string, birthday time.Time) (uuid.UUID, error) {
			if login != "john" || email != "john@example.com" || name != "John Doe" {
				t.Fatalf("unexpected create payload: %q %q %q", login, email, name)
			}
			if hash == "" || hash == "super-secret" {
				t.Fatalf("expected hashed password, got %q", hash)
			}
			if !birthday.Equal(expectedBirthday) {
				t.Fatalf("unexpected birthday: %v", birthday)
			}
			return userID, nil
		},
		getByLoginFn: func(_ context.Context, _ string) (*domain.UserWithPassword, error) {
			t.Fatal("GetByLogin should not be called in Register")
			return nil, nil
		},
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.User, error) {
			t.Fatal("GetByID should not be called in Register")
			return nil, nil
		},
	}

	h := newAuthHandlers("register-secret", repo)
	body := `{"login":"john","email":"john@example.com","name":"John Doe","password":"super-secret","birthday":"1995-10-15"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" {
		t.Fatal("expected token in response")
	}

	parsedID, err := authinfra.ParseToken(resp.Token, "register-secret")
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if parsedID != userID {
		t.Fatalf("unexpected token subject: got %s want %s", parsedID, userID)
	}
}

func TestRegisterHandlerInvalidRequest(t *testing.T) {
	repo := &handlersRepoStub{
		createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
			t.Fatal("Create should not be called on invalid request")
			return uuid.Nil, nil
		},
		getByLoginFn: func(_ context.Context, _ string) (*domain.UserWithPassword, error) {
			t.Fatal("GetByLogin should not be called")
			return nil, nil
		},
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.User, error) {
			t.Fatal("GetByID should not be called")
			return nil, nil
		},
	}
	h := newAuthHandlers("secret", repo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewBufferString(`{"login":"john","unknown":"x"}`))
	rec := httptest.NewRecorder()

	h.Register(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid request") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestLoginHandlerInvalidCredentials(t *testing.T) {
	repo := &handlersRepoStub{
		createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
			t.Fatal("Create should not be called")
			return uuid.Nil, nil
		},
		getByLoginFn: func(_ context.Context, _ string) (*domain.UserWithPassword, error) {
			return nil, domain.ErrUserNotFound
		},
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.User, error) {
			t.Fatal("GetByID should not be called")
			return nil, nil
		},
	}
	h := newAuthHandlers("secret", repo)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"login":"john","password":"bad"}`))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid credentials") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestGetMeHandlerUnauthorizedWithoutMiddlewareContext(t *testing.T) {
	h := newAuthHandlers("secret", &handlersRepoStub{
		createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
			t.Fatal("Create should not be called")
			return uuid.Nil, nil
		},
		getByLoginFn: func(_ context.Context, _ string) (*domain.UserWithPassword, error) {
			t.Fatal("GetByLogin should not be called")
			return nil, nil
		},
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.User, error) {
			t.Fatal("GetByID should not be called")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	rec := httptest.NewRecorder()
	h.GetMe(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "unauthorized") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestGetMeHandlerSuccessThroughMiddleware(t *testing.T) {
	userID := uuid.New()
	birthday := time.Date(1995, time.October, 15, 0, 0, 0, 0, time.UTC)
	repo := &handlersRepoStub{
		createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
			t.Fatal("Create should not be called")
			return uuid.Nil, nil
		},
		getByLoginFn: func(_ context.Context, _ string) (*domain.UserWithPassword, error) {
			t.Fatal("GetByLogin should not be called")
			return nil, nil
		},
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.User, error) {
			if id != userID {
				t.Fatalf("unexpected user id: %s", id)
			}
			return &domain.User{
				ID:       userID,
				Login:    "john",
				Email:    "john@example.com",
				Name:     "John Doe",
				Birthday: birthday,
			}, nil
		},
	}

	secret := "secret"
	h := newAuthHandlers(secret, repo)
	handler := httpmw.JWTMiddleware(secret)(http.HandlerFunc(h.GetMe))

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp UserResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != userID.String() {
		t.Fatalf("unexpected id: %s", resp.ID)
	}
	if resp.Login != "john" || resp.Email != "john@example.com" || resp.Name != "John Doe" {
		t.Fatalf("unexpected profile response: %+v", resp)
	}
	if resp.Birthday == nil {
		t.Fatal("expected birthday in response")
	}
	if !resp.Birthday.Time().Equal(birthday) {
		t.Fatalf("unexpected birthday: %v", resp.Birthday.Time())
	}
}

func TestGetMeHandlerNotFound(t *testing.T) {
	userID := uuid.New()
	repo := &handlersRepoStub{
		createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
			t.Fatal("Create should not be called")
			return uuid.Nil, nil
		},
		getByLoginFn: func(_ context.Context, _ string) (*domain.UserWithPassword, error) {
			t.Fatal("GetByLogin should not be called")
			return nil, nil
		},
		getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.User, error) {
			if id != userID {
				t.Fatalf("unexpected user id: %s", id)
			}
			return nil, domain.ErrUserNotFound
		},
	}

	secret := "secret"
	h := newAuthHandlers(secret, repo)
	handler := httpmw.JWTMiddleware(secret)(http.HandlerFunc(h.GetMe))

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not found") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestBirthdayPtr(t *testing.T) {
	if birthdayPtr(time.Time{}) != nil {
		t.Fatal("expected nil for zero birthday")
	}

	src := time.Date(2000, time.January, 2, 0, 0, 0, 0, time.UTC)
	got := birthdayPtr(src)
	if got == nil {
		t.Fatal("expected non-nil birthday pointer")
	}
	if !got.Time().Equal(src) {
		t.Fatalf("unexpected date: %v", got.Time())
	}

	var _ *httpdto.Date = got
}
