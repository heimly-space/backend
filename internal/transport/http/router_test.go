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

type routerRefreshStoreStub struct {
	storeFn  func(ctx context.Context, userID uuid.UUID, refreshJTI string, ttl time.Duration) error
	rotateFn func(ctx context.Context, userID uuid.UUID, oldRefreshJTI, newRefreshJTI string, ttl time.Duration) error
	revokeFn func(ctx context.Context, userID uuid.UUID, refreshJTI string) error
}

func (s *routerRefreshStoreStub) Store(
	ctx context.Context,
	userID uuid.UUID,
	refreshJTI string,
	ttl time.Duration,
) error {
	if s.storeFn == nil {
		return errors.New("unexpected Store call")
	}
	return s.storeFn(ctx, userID, refreshJTI, ttl)
}

func (s *routerRefreshStoreStub) Rotate(
	ctx context.Context,
	userID uuid.UUID,
	oldRefreshJTI, newRefreshJTI string,
	ttl time.Duration,
) error {
	if s.rotateFn == nil {
		return errors.New("unexpected Rotate call")
	}
	return s.rotateFn(ctx, userID, oldRefreshJTI, newRefreshJTI, ttl)
}

func (s *routerRefreshStoreStub) Revoke(ctx context.Context, userID uuid.UUID, refreshJTI string) error {
	if s.revokeFn == nil {
		return errors.New("unexpected Revoke call")
	}
	return s.revokeFn(ctx, userID, refreshJTI)
}

type routerAccessStoreStub struct {
	storeFn    func(ctx context.Context, jti string, userID uuid.UUID, ttl time.Duration) error
	isActiveFn func(ctx context.Context, jti string, userID uuid.UUID) (bool, error)
	revokeFn   func(ctx context.Context, jti string) error
}

func (s *routerAccessStoreStub) StoreAccessToken(
	ctx context.Context,
	jti string,
	userID uuid.UUID,
	ttl time.Duration,
) error {
	if s.storeFn == nil {
		return errors.New("unexpected StoreAccessToken call")
	}
	return s.storeFn(ctx, jti, userID, ttl)
}

func (s *routerAccessStoreStub) IsAccessTokenActive(
	ctx context.Context,
	jti string,
	userID uuid.UUID,
) (bool, error) {
	if s.isActiveFn == nil {
		return false, errors.New("unexpected IsAccessTokenActive call")
	}
	return s.isActiveFn(ctx, jti, userID)
}

func (s *routerAccessStoreStub) RevokeAccessToken(ctx context.Context, jti string) error {
	if s.revokeFn == nil {
		return errors.New("unexpected RevokeAccessToken call")
	}
	return s.revokeFn(ctx, jti)
}

func newTestRouter(
	secret string,
	repo domain.Repository,
	accessStore domain.AccessTokenStore,
	refreshStore domain.RefreshTokenStore,
) nethttp.Handler {
	handlers := &usershttp.AuthHandlers{
		Users: &domain.Service{
			Repo:            repo,
			AccessTokens:    accessStore,
			RefreshTokens:   refreshStore,
			JWTSecret:       secret,
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		},
	}
	return NewRouter(handlers, &cfg.Config{JWTSecret: secret})
}

func TestRouterHealth(t *testing.T) {
	router := newTestRouter("secret", &routerRepoStub{}, &routerAccessStoreStub{}, &routerRefreshStoreStub{})

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
	passwordHash, err := authinfra.HashPassword("secret-pass")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	var gotRefreshJTI string
	router := newTestRouter(
		"secret",
		&routerRepoStub{
			getByLoginFn: func(_ context.Context, login string) (*domain.UserWithPassword, error) {
				if login != "john" {
					t.Fatalf("unexpected login: %s", login)
				}
				return &domain.UserWithPassword{
					User:           domain.User{ID: userID, Login: "john"},
					HashedPassword: passwordHash,
				}, nil
			},
		},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, gotUserID uuid.UUID, _ time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected user id: %s", gotUserID)
				}
				return nil
			},
			isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
				t.Fatal("IsAccessTokenActive should not be called in login route")
				return false, nil
			},
		},
		&routerRefreshStoreStub{
			storeFn: func(_ context.Context, gotUserID uuid.UUID, refreshJTI string, _ time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected user id: %s", gotUserID)
				}
				gotRefreshJTI = refreshJTI
				return nil
			},
		},
	)

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
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
	refreshClaims, err := authinfra.ParseRefreshTokenClaims(resp.RefreshToken, "secret")
	if err != nil {
		t.Fatalf("refresh token should be valid: %v", err)
	}
	if gotRefreshJTI != refreshClaims.JTI {
		t.Fatalf("stored refresh jti mismatch: got %s want %s", gotRefreshJTI, refreshClaims.JTI)
	}
	if refreshClaims.UserID != userID {
		t.Fatalf("unexpected refresh token subject: got %s want %s", refreshClaims.UserID, userID)
	}
	if _, err := authinfra.ParseToken(resp.AccessToken, "secret"); err != nil {
		t.Fatalf("access token should be valid: %v", err)
	}
}

func TestRouterRefreshRoute(t *testing.T) {
	userID := uuid.New()
	oldRefresh, oldRefreshJTI, err := authinfra.GenerateRefreshToken(userID, "secret", 24*time.Hour)
	if err != nil {
		t.Fatalf("generate old refresh token: %v", err)
	}
	var gotRotateUserID uuid.UUID
	var gotOldJTI string
	var gotNewJTI string

	router := newTestRouter(
		"secret",
		&routerRepoStub{},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, gotUserID uuid.UUID, _ time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected user id: %s", gotUserID)
				}
				return nil
			},
			isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
				t.Fatal("IsAccessTokenActive should not be called in refresh route")
				return false, nil
			},
		},
		&routerRefreshStoreStub{
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
		},
	)

	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(`{"refresh_token":"`+oldRefresh+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if gotRotateUserID != userID {
		t.Fatalf("unexpected rotate user id: got %s want %s", gotRotateUserID, userID)
	}
	if gotOldJTI != oldRefreshJTI {
		t.Fatalf("old refresh jti mismatch: got %s want %s", gotOldJTI, oldRefreshJTI)
	}

	var resp usershttp.AuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatal("expected access and refresh tokens")
	}
	newRefreshClaims, err := authinfra.ParseRefreshTokenClaims(resp.RefreshToken, "secret")
	if err != nil {
		t.Fatalf("refresh token should be valid: %v", err)
	}
	if gotNewJTI != newRefreshClaims.JTI {
		t.Fatalf("new refresh jti mismatch: got %s want %s", gotNewJTI, newRefreshClaims.JTI)
	}
	if newRefreshClaims.UserID != userID {
		t.Fatalf("unexpected refresh token subject: got %s want %s", newRefreshClaims.UserID, userID)
	}
}

func TestRouterLogoutRoute(t *testing.T) {
	userID := uuid.New()
	secret := "secret"
	var accessToken string
	refreshToken, refreshJTI, err := authinfra.GenerateRefreshToken(userID, secret, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}
	var gotRevokedRefreshUserID uuid.UUID
	var gotRevokedRefreshJTI string
	var gotRevokedJTI string

	router := newTestRouter(
		secret,
		&routerRepoStub{},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
				t.Fatal("StoreAccessToken should not be called in logout route")
				return nil
			},
			isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
				t.Fatal("IsAccessTokenActive should not be called in logout route")
				return false, nil
			},
			revokeFn: func(_ context.Context, jti string) error {
				gotRevokedJTI = jti
				return nil
			},
		},
		&routerRefreshStoreStub{
			revokeFn: func(_ context.Context, gotUserID uuid.UUID, gotRefreshJTI string) error {
				gotRevokedRefreshUserID = gotUserID
				gotRevokedRefreshJTI = gotRefreshJTI
				return nil
			},
		},
	)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	accessToken = token
	claims, err := authinfra.ParseTokenClaims(accessToken, secret)
	if err != nil {
		t.Fatalf("parse token claims: %v", err)
	}

	req := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/v1/auth/logout",
		bytes.NewBufferString(`{"refresh_token":"`+refreshToken+`"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != nethttp.StatusNoContent {
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

func TestRouterUsersMeRouteProtected(t *testing.T) {
	router := newTestRouter(
		"secret",
		&routerRepoStub{},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
				t.Fatal("StoreAccessToken should not be called")
				return nil
			},
			isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
				t.Fatal("IsAccessTokenActive should not be called without bearer token")
				return false, nil
			},
		},
		&routerRefreshStoreStub{},
	)

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
	var accessToken string
	router := newTestRouter(
		secret,
		&routerRepoStub{
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
		},
		&routerAccessStoreStub{
			storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
				t.Fatal("StoreAccessToken should not be called in protected route")
				return nil
			},
			isActiveFn: func(_ context.Context, jti string, gotUserID uuid.UUID) (bool, error) {
				claims, err := authinfra.ParseTokenClaims(accessToken, secret)
				if err != nil {
					t.Fatalf("parse token claims: %v", err)
				}
				return claims.JTI == jti && gotUserID == userID, nil
			},
		},
		&routerRefreshStoreStub{},
	)

	token, err := authinfra.GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	accessToken = token

	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
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
