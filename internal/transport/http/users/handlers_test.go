package users

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

type handlersRefreshStoreStub struct {
	storeFn  func(ctx context.Context, userID uuid.UUID, refreshJTI string, ttl time.Duration) error
	rotateFn func(ctx context.Context, userID uuid.UUID, oldRefreshJTI, newRefreshJTI string, ttl time.Duration) error
	revokeFn func(ctx context.Context, userID uuid.UUID, refreshJTI string) error
}

func (s *handlersRefreshStoreStub) Store(
	ctx context.Context,
	userID uuid.UUID,
	refreshJTI string,
	ttl time.Duration,
) error {
	return s.storeFn(ctx, userID, refreshJTI, ttl)
}

func (s *handlersRefreshStoreStub) Rotate(
	ctx context.Context,
	userID uuid.UUID,
	oldRefreshJTI, newRefreshJTI string,
	ttl time.Duration,
) error {
	return s.rotateFn(ctx, userID, oldRefreshJTI, newRefreshJTI, ttl)
}

func (s *handlersRefreshStoreStub) Revoke(ctx context.Context, userID uuid.UUID, refreshJTI string) error {
	if s.revokeFn == nil {
		return errors.New("unexpected Revoke call")
	}
	return s.revokeFn(ctx, userID, refreshJTI)
}

type handlersAccessStoreStub struct {
	storeFn    func(ctx context.Context, jti string, userID uuid.UUID, ttl time.Duration) error
	isActiveFn func(ctx context.Context, jti string, userID uuid.UUID) (bool, error)
	revokeFn   func(ctx context.Context, jti string) error
}

func (s *handlersAccessStoreStub) StoreAccessToken(
	ctx context.Context,
	jti string,
	userID uuid.UUID,
	ttl time.Duration,
) error {
	return s.storeFn(ctx, jti, userID, ttl)
}

func (s *handlersAccessStoreStub) IsAccessTokenActive(
	ctx context.Context,
	jti string,
	userID uuid.UUID,
) (bool, error) {
	return s.isActiveFn(ctx, jti, userID)
}

func (s *handlersAccessStoreStub) RevokeAccessToken(ctx context.Context, jti string) error {
	if s.revokeFn == nil {
		return errors.New("unexpected RevokeAccessToken call")
	}
	return s.revokeFn(ctx, jti)
}

func newAuthHandlers(
	secret string,
	repo domain.Repository,
	accessStore domain.AccessTokenStore,
	refreshStore domain.RefreshTokenStore,
) *AuthHandlers {
	return &AuthHandlers{
		Users: &domain.Service{
			Repo:            repo,
			AccessTokens:    accessStore,
			RefreshTokens:   refreshStore,
			JWTSecret:       secret,
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		},
	}
}

func TestRegisterHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	expectedBirthday := time.Date(1995, time.October, 15, 0, 0, 0, 0, time.UTC)
	var gotRefreshJTI string
	var gotAccessJTI string

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
	refreshStore := &handlersRefreshStoreStub{
		storeFn: func(_ context.Context, gotUserID uuid.UUID, refreshJTI string, _ time.Duration) error {
			if gotUserID != userID {
				t.Fatalf("unexpected refresh token user id: %s", gotUserID)
			}
			gotRefreshJTI = refreshJTI
			return nil
		},
		rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
			t.Fatal("Rotate should not be called in Register")
			return nil
		},
	}
	accessStore := &handlersAccessStoreStub{
		storeFn: func(_ context.Context, jti string, gotUserID uuid.UUID, ttl time.Duration) error {
			if gotUserID != userID {
				t.Fatalf("unexpected access token user id: %s", gotUserID)
			}
			if ttl != time.Hour {
				t.Fatalf("unexpected access ttl: %v", ttl)
			}
			gotAccessJTI = jti
			return nil
		},
		isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			t.Fatal("IsAccessTokenActive should not be called")
			return false, nil
		},
	}

	h := newAuthHandlers("register-secret", repo, accessStore, refreshStore)
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
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
	refreshClaims, err := authinfra.ParseRefreshTokenClaims(resp.RefreshToken, "register-secret")
	if err != nil {
		t.Fatalf("parse refresh token claims: %v", err)
	}
	if gotRefreshJTI != refreshClaims.JTI {
		t.Fatalf("unexpected refresh token jti: got %s want %s", gotRefreshJTI, refreshClaims.JTI)
	}
	if refreshClaims.UserID != userID {
		t.Fatalf("unexpected refresh token subject: got %s want %s", refreshClaims.UserID, userID)
	}

	claims, err := authinfra.ParseTokenClaims(resp.AccessToken, "register-secret")
	if err != nil {
		t.Fatalf("parse access token claims: %v", err)
	}
	if claims.UserID != userID {
		t.Fatalf("unexpected token subject: got %s want %s", claims.UserID, userID)
	}
	if claims.JTI != gotAccessJTI {
		t.Fatalf("unexpected access token jti: got %s want %s", claims.JTI, gotAccessJTI)
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
	refreshStore := &handlersRefreshStoreStub{
		storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
			t.Fatal("Store should not be called")
			return nil
		},
		rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
			t.Fatal("Rotate should not be called")
			return nil
		},
	}
	accessStore := &handlersAccessStoreStub{
		storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
			t.Fatal("StoreAccessToken should not be called")
			return nil
		},
		isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			t.Fatal("IsAccessTokenActive should not be called")
			return false, nil
		},
	}
	h := newAuthHandlers("secret", repo, accessStore, refreshStore)

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
	refreshStore := &handlersRefreshStoreStub{
		storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
			t.Fatal("Store should not be called")
			return nil
		},
		rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
			t.Fatal("Rotate should not be called")
			return nil
		},
	}
	accessStore := &handlersAccessStoreStub{
		storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
			t.Fatal("StoreAccessToken should not be called")
			return nil
		},
		isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			t.Fatal("IsAccessTokenActive should not be called")
			return false, nil
		},
	}
	h := newAuthHandlers("secret", repo, accessStore, refreshStore)

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

func TestRefreshHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	oldRefreshToken, oldRefreshJTI, err := authinfra.GenerateRefreshToken(userID, "secret", 24*time.Hour)
	if err != nil {
		t.Fatalf("generate old refresh token: %v", err)
	}
	var gotRotateUserID uuid.UUID
	var gotOldJTI string
	var gotNewJTI string
	var gotAccessJTI string

	repo := &handlersRepoStub{
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
	}
	refreshStore := &handlersRefreshStoreStub{
		storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
			t.Fatal("Store should not be called")
			return nil
		},
		rotateFn: func(
			_ context.Context,
			gotUserID uuid.UUID,
			oldRefreshJTI, newRefreshJTI string,
			_ time.Duration,
		) error {
			gotRotateUserID = gotUserID
			gotOldJTI = oldRefreshJTI
			gotNewJTI = newRefreshJTI
			return nil
		},
	}
	accessStore := &handlersAccessStoreStub{
		storeFn: func(_ context.Context, jti string, gotUserID uuid.UUID, ttl time.Duration) error {
			if gotUserID != userID {
				t.Fatalf("unexpected access token user id: %s", gotUserID)
			}
			if ttl != time.Hour {
				t.Fatalf("unexpected access ttl: %v", ttl)
			}
			gotAccessJTI = jti
			return nil
		},
		isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			t.Fatal("IsAccessTokenActive should not be called")
			return false, nil
		},
	}
	h := newAuthHandlers("secret", repo, accessStore, refreshStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(`{"refresh_token":"`+oldRefreshToken+`"}`))
	rec := httptest.NewRecorder()
	h.Refresh(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if gotRotateUserID != userID {
		t.Fatalf("unexpected rotate user id: got %s want %s", gotRotateUserID, userID)
	}
	if gotOldJTI != oldRefreshJTI {
		t.Fatalf("old refresh jti mismatch: got %s want %s", gotOldJTI, oldRefreshJTI)
	}

	var resp AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
	claims, err := authinfra.ParseTokenClaims(resp.AccessToken, "secret")
	if err != nil {
		t.Fatalf("parse access token claims: %v", err)
	}
	if claims.JTI != gotAccessJTI {
		t.Fatalf("unexpected access token jti: got %s want %s", claims.JTI, gotAccessJTI)
	}
	newRefreshClaims, err := authinfra.ParseRefreshTokenClaims(resp.RefreshToken, "secret")
	if err != nil {
		t.Fatalf("parse refresh token claims: %v", err)
	}
	if gotNewJTI != newRefreshClaims.JTI {
		t.Fatalf("new refresh jti mismatch: got %s want %s", gotNewJTI, newRefreshClaims.JTI)
	}
	if newRefreshClaims.UserID != userID {
		t.Fatalf("unexpected refresh token subject: got %s want %s", newRefreshClaims.UserID, userID)
	}
}

func TestRefreshHandlerInvalidToken(t *testing.T) {
	repo := &handlersRepoStub{
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
	}
	refreshStore := &handlersRefreshStoreStub{
		storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
			t.Fatal("Store should not be called")
			return nil
		},
		rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
			t.Fatal("Rotate should not be called for invalid refresh JWT")
			return nil
		},
	}
	accessStore := &handlersAccessStoreStub{
		storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
			t.Fatal("StoreAccessToken should not be called")
			return nil
		},
		isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			t.Fatal("IsAccessTokenActive should not be called")
			return false, nil
		},
	}
	h := newAuthHandlers("secret", repo, accessStore, refreshStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(`{"refresh_token":"bad-token"}`))
	rec := httptest.NewRecorder()
	h.Refresh(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid refresh token") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestLogoutHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	secret := "secret"
	accessToken, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	claims, err := authinfra.ParseTokenClaims(accessToken, secret)
	if err != nil {
		t.Fatalf("parse token claims: %v", err)
	}
	refreshToken, refreshJTI, err := authinfra.GenerateRefreshToken(userID, secret, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	repo := &handlersRepoStub{
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
	}
	var gotRevokedRefreshUserID uuid.UUID
	var gotRevokedRefreshJTI string
	var gotRevokedJTI string
	refreshStore := &handlersRefreshStoreStub{
		storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
			t.Fatal("Store should not be called")
			return nil
		},
		rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
			t.Fatal("Rotate should not be called")
			return nil
		},
		revokeFn: func(_ context.Context, gotUserID uuid.UUID, gotRefreshJTI string) error {
			gotRevokedRefreshUserID = gotUserID
			gotRevokedRefreshJTI = gotRefreshJTI
			return nil
		},
	}
	accessStore := &handlersAccessStoreStub{
		storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
			t.Fatal("StoreAccessToken should not be called")
			return nil
		},
		isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			t.Fatal("IsAccessTokenActive should not be called")
			return false, nil
		},
		revokeFn: func(_ context.Context, jti string) error {
			gotRevokedJTI = jti
			return nil
		},
	}
	h := newAuthHandlers(secret, repo, accessStore, refreshStore)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/logout",
		bytes.NewBufferString(`{"refresh_token":"`+refreshToken+`"}`),
	)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d (%s)", rec.Code, rec.Body.String())
	}
	if gotRevokedRefreshUserID != userID {
		t.Fatalf("unexpected revoked refresh user id: got %s want %s", gotRevokedRefreshUserID, userID)
	}
	if gotRevokedRefreshJTI != refreshJTI {
		t.Fatalf("unexpected revoked refresh jti: got %s want %s", gotRevokedRefreshJTI, refreshJTI)
	}
	if gotRevokedJTI != claims.JTI {
		t.Fatalf("unexpected revoked access jti: %s", gotRevokedJTI)
	}
}

func TestLogoutHandlerInvalidToken(t *testing.T) {
	repo := &handlersRepoStub{
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
	}
	refreshStore := &handlersRefreshStoreStub{
		storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
			t.Fatal("Store should not be called")
			return nil
		},
		rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
			t.Fatal("Rotate should not be called")
			return nil
		},
		revokeFn: func(_ context.Context, _ uuid.UUID, _ string) error {
			t.Fatal("Revoke should not be called")
			return nil
		},
	}
	accessStore := &handlersAccessStoreStub{
		storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
			t.Fatal("StoreAccessToken should not be called")
			return nil
		},
		isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			t.Fatal("IsAccessTokenActive should not be called")
			return false, nil
		},
		revokeFn: func(_ context.Context, _ string) error {
			t.Fatal("RevokeAccessToken should not be called")
			return nil
		},
	}
	h := newAuthHandlers("secret", repo, accessStore, refreshStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBufferString(`{"refresh_token":"bye-refresh"}`))
	req.Header.Set("Authorization", "Bearer bad-token")
	rec := httptest.NewRecorder()
	h.Logout(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGetMeHandlerUnauthorizedWithoutMiddlewareContext(t *testing.T) {
	repo := &handlersRepoStub{
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
	}
	refreshStore := &handlersRefreshStoreStub{
		storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
			t.Fatal("Store should not be called")
			return nil
		},
		rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
			t.Fatal("Rotate should not be called")
			return nil
		},
	}
	accessStore := &handlersAccessStoreStub{
		storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
			t.Fatal("StoreAccessToken should not be called")
			return nil
		},
		isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			t.Fatal("IsAccessTokenActive should not be called")
			return false, nil
		},
	}
	h := newAuthHandlers("secret", repo, accessStore, refreshStore)

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
	secret := "secret"
	var accessToken string
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
	refreshStore := &handlersRefreshStoreStub{
		storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
			t.Fatal("Store should not be called")
			return nil
		},
		rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
			t.Fatal("Rotate should not be called")
			return nil
		},
	}
	accessStore := &handlersAccessStoreStub{
		storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
			t.Fatal("StoreAccessToken should not be called")
			return nil
		},
		isActiveFn: func(_ context.Context, jti string, gotUserID uuid.UUID) (bool, error) {
			claims, err := authinfra.ParseTokenClaims(accessToken, secret)
			if err != nil {
				t.Fatalf("parse access token claims: %v", err)
			}
			return jti == claims.JTI && gotUserID == userID, nil
		},
	}

	h := newAuthHandlers(secret, repo, accessStore, refreshStore)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	accessToken = token
	handler := httpmw.JWTMiddleware(secret, accessStore)(http.HandlerFunc(h.GetMe))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
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
	refreshStore := &handlersRefreshStoreStub{
		storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
			t.Fatal("Store should not be called")
			return nil
		},
		rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
			t.Fatal("Rotate should not be called")
			return nil
		},
	}
	accessStore := &handlersAccessStoreStub{
		storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
			t.Fatal("StoreAccessToken should not be called")
			return nil
		},
		isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
			return true, nil
		},
	}

	secret := "secret"
	h := newAuthHandlers(secret, repo, accessStore, refreshStore)
	handler := httpmw.JWTMiddleware(secret, accessStore)(http.HandlerFunc(h.GetMe))

	accessToken, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
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
