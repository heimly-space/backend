package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	authinfra "heimly.space/backend/internal/infra/auth"
)

type repoStub struct {
	createFn     func(ctx context.Context, login, email, name, hash string, birthday time.Time) (uuid.UUID, error)
	getByLoginFn func(ctx context.Context, login string) (*UserWithPassword, error)
	getByIDFn    func(ctx context.Context, id uuid.UUID) (*User, error)
}

func (r *repoStub) Create(
	ctx context.Context,
	login, email, name, hash string,
	birthday time.Time,
) (uuid.UUID, error) {
	return r.createFn(ctx, login, email, name, hash, birthday)
}

func (r *repoStub) GetByLogin(ctx context.Context, login string) (*UserWithPassword, error) {
	return r.getByLoginFn(ctx, login)
}

func (r *repoStub) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return r.getByIDFn(ctx, id)
}

type refreshStoreStub struct {
	storeFn  func(ctx context.Context, userID uuid.UUID, refreshJTI string, ttl time.Duration) error
	rotateFn func(ctx context.Context, userID uuid.UUID, oldRefreshJTI, newRefreshJTI string, ttl time.Duration) error
	revokeFn func(ctx context.Context, userID uuid.UUID, refreshJTI string) error
}

func (s *refreshStoreStub) Store(
	ctx context.Context,
	userID uuid.UUID,
	refreshJTI string,
	ttl time.Duration,
) error {
	return s.storeFn(ctx, userID, refreshJTI, ttl)
}

func (s *refreshStoreStub) Rotate(
	ctx context.Context,
	userID uuid.UUID,
	oldRefreshJTI, newRefreshJTI string,
	ttl time.Duration,
) error {
	return s.rotateFn(ctx, userID, oldRefreshJTI, newRefreshJTI, ttl)
}

func (s *refreshStoreStub) Revoke(ctx context.Context, userID uuid.UUID, refreshJTI string) error {
	if s.revokeFn == nil {
		return errors.New("unexpected Revoke call")
	}
	return s.revokeFn(ctx, userID, refreshJTI)
}

type accessStoreStub struct {
	storeFn    func(ctx context.Context, jti string, userID uuid.UUID, ttl time.Duration) error
	isActiveFn func(ctx context.Context, jti string, userID uuid.UUID) (bool, error)
	revokeFn   func(ctx context.Context, jti string) error
}

func (s *accessStoreStub) StoreAccessToken(
	ctx context.Context,
	jti string,
	userID uuid.UUID,
	ttl time.Duration,
) error {
	return s.storeFn(ctx, jti, userID, ttl)
}

func (s *accessStoreStub) IsAccessTokenActive(
	ctx context.Context,
	jti string,
	userID uuid.UUID,
) (bool, error) {
	return s.isActiveFn(ctx, jti, userID)
}

func (s *accessStoreStub) RevokeAccessToken(ctx context.Context, jti string) error {
	if s.revokeFn == nil {
		return errors.New("unexpected RevokeAccessToken call")
	}
	return s.revokeFn(ctx, jti)
}

func TestServiceRegister(t *testing.T) {
	userID := uuid.New()
	birthday := time.Date(1995, time.October, 15, 0, 0, 0, 0, time.UTC)
	var gotPasswordHash string
	var gotRefreshJTI string
	var gotAccessJTI string

	svc := &Service{
		Repo: &repoStub{
			createFn: func(_ context.Context, login, email, name, hash string, gotBirthday time.Time) (uuid.UUID, error) {
				if login != "john" || email != "john@example.com" || name != "John Doe" {
					t.Fatalf("unexpected create params: %q %q %q", login, email, name)
				}
				if !gotBirthday.Equal(birthday) {
					t.Fatalf("unexpected birthday: %v", gotBirthday)
				}
				gotPasswordHash = hash
				return userID, nil
			},
			getByLoginFn: func(_ context.Context, _ string) (*UserWithPassword, error) {
				t.Fatal("GetByLogin should not be called in Register")
				return nil, nil
			},
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*User, error) {
				t.Fatal("GetByID should not be called in Register")
				return nil, nil
			},
		},
		AccessTokens: &accessStoreStub{
			storeFn: func(_ context.Context, jti string, gotUserID uuid.UUID, ttl time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected access user id: %s", gotUserID)
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
		},
		RefreshTokens: &refreshStoreStub{
			storeFn: func(_ context.Context, gotUserID uuid.UUID, refreshJTI string, ttl time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected refresh user id: %s", gotUserID)
				}
				if ttl != 24*time.Hour {
					t.Fatalf("unexpected refresh ttl: %v", ttl)
				}
				gotRefreshJTI = refreshJTI
				return nil
			},
			rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
				t.Fatal("Rotate should not be called in Register")
				return nil
			},
		},
		JWTSecret:       "register-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}

	tokens, err := svc.Register(context.Background(), "john", "john@example.com", "John Doe", "secret-pass", birthday)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if gotPasswordHash == "" || gotPasswordHash == "secret-pass" {
		t.Fatal("expected password hash")
	}
	if err := authinfra.CheckPassword(gotPasswordHash, "secret-pass"); err != nil {
		t.Fatalf("password hash mismatch: %v", err)
	}
	refreshClaims, err := authinfra.ParseRefreshTokenClaims(tokens.RefreshToken, "register-secret")
	if err != nil {
		t.Fatalf("parse refresh token claims: %v", err)
	}
	if gotRefreshJTI != refreshClaims.JTI {
		t.Fatalf("refresh jti mismatch: got %s want %s", gotRefreshJTI, refreshClaims.JTI)
	}
	if refreshClaims.UserID != userID {
		t.Fatalf("unexpected refresh token subject: got %s want %s", refreshClaims.UserID, userID)
	}
	claims, err := authinfra.ParseTokenClaims(tokens.AccessToken, "register-secret")
	if err != nil {
		t.Fatalf("parse access token claims: %v", err)
	}
	if gotAccessJTI != claims.JTI {
		t.Fatalf("access jti mismatch: got %s want %s", gotAccessJTI, claims.JTI)
	}

	if claims.UserID != userID {
		t.Fatalf("unexpected token subject: got %s want %s", claims.UserID, userID)
	}
}

func TestServiceLoginSuccess(t *testing.T) {
	userID := uuid.New()
	passwordHash, err := authinfra.HashPassword("secret-pass")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	var gotRefreshJTI string
	var gotAccessJTI string
	svc := &Service{
		Repo: &repoStub{
			createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
				t.Fatal("Create should not be called in Login")
				return uuid.Nil, nil
			},
			getByLoginFn: func(_ context.Context, login string) (*UserWithPassword, error) {
				if login != "john" {
					t.Fatalf("unexpected login: %q", login)
				}
				return &UserWithPassword{
					User:           User{ID: userID, Login: "john"},
					HashedPassword: passwordHash,
				}, nil
			},
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*User, error) {
				t.Fatal("GetByID should not be called in Login")
				return nil, nil
			},
		},
		AccessTokens: &accessStoreStub{
			storeFn: func(_ context.Context, jti string, gotUserID uuid.UUID, ttl time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected access user id: %s", gotUserID)
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
		},
		RefreshTokens: &refreshStoreStub{
			storeFn: func(_ context.Context, gotUserID uuid.UUID, refreshJTI string, ttl time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected user id: %s", gotUserID)
				}
				if ttl != 24*time.Hour {
					t.Fatalf("unexpected refresh ttl: %v", ttl)
				}
				gotRefreshJTI = refreshJTI
				return nil
			},
			rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
				t.Fatal("Rotate should not be called in Login")
				return nil
			},
		},
		JWTSecret:       "login-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}

	tokens, err := svc.Login(context.Background(), "john", "secret-pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	refreshClaims, err := authinfra.ParseRefreshTokenClaims(tokens.RefreshToken, "login-secret")
	if err != nil {
		t.Fatalf("parse refresh token claims: %v", err)
	}
	if gotRefreshJTI != refreshClaims.JTI {
		t.Fatalf("refresh jti mismatch: got %s want %s", gotRefreshJTI, refreshClaims.JTI)
	}
	if refreshClaims.UserID != userID {
		t.Fatalf("unexpected refresh token subject: got %s want %s", refreshClaims.UserID, userID)
	}
	claims, err := authinfra.ParseTokenClaims(tokens.AccessToken, "login-secret")
	if err != nil {
		t.Fatalf("parse access token claims: %v", err)
	}
	if gotAccessJTI != claims.JTI {
		t.Fatalf("access jti mismatch: got %s want %s", gotAccessJTI, claims.JTI)
	}

	if claims.UserID != userID {
		t.Fatalf("unexpected token subject: got %s want %s", claims.UserID, userID)
	}
}

func TestServiceLoginInvalidCredentials(t *testing.T) {
	t.Run("user-not-found", func(t *testing.T) {
		svc := &Service{
			Repo: &repoStub{
				createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
					t.Fatal("Create should not be called in Login")
					return uuid.Nil, nil
				},
				getByLoginFn: func(_ context.Context, _ string) (*UserWithPassword, error) {
					return nil, ErrUserNotFound
				},
				getByIDFn: func(_ context.Context, _ uuid.UUID) (*User, error) {
					t.Fatal("GetByID should not be called in Login")
					return nil, nil
				},
			},
			AccessTokens: &accessStoreStub{
				storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
					t.Fatal("StoreAccessToken should not be called")
					return nil
				},
				isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
					t.Fatal("IsAccessTokenActive should not be called")
					return false, nil
				},
			},
			RefreshTokens: &refreshStoreStub{
				storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
					t.Fatal("Store should not be called")
					return nil
				},
				rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
					t.Fatal("Rotate should not be called")
					return nil
				},
			},
			JWTSecret:       "login-secret",
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		}

		_, err := svc.Login(context.Background(), "john", "secret-pass")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("wrong-password", func(t *testing.T) {
		passwordHash, err := authinfra.HashPassword("right-pass")
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}

		svc := &Service{
			Repo: &repoStub{
				createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
					t.Fatal("Create should not be called in Login")
					return uuid.Nil, nil
				},
				getByLoginFn: func(_ context.Context, _ string) (*UserWithPassword, error) {
					return &UserWithPassword{
						User:           User{ID: uuid.New()},
						HashedPassword: passwordHash,
					}, nil
				},
				getByIDFn: func(_ context.Context, _ uuid.UUID) (*User, error) {
					t.Fatal("GetByID should not be called in Login")
					return nil, nil
				},
			},
			AccessTokens: &accessStoreStub{
				storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
					t.Fatal("StoreAccessToken should not be called")
					return nil
				},
				isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
					t.Fatal("IsAccessTokenActive should not be called")
					return false, nil
				},
			},
			RefreshTokens: &refreshStoreStub{
				storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
					t.Fatal("Store should not be called")
					return nil
				},
				rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
					t.Fatal("Rotate should not be called")
					return nil
				},
			},
			JWTSecret:       "login-secret",
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
		}

		_, err = svc.Login(context.Background(), "john", "wrong-pass")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestServiceRefresh(t *testing.T) {
	userID := uuid.New()
	oldRefreshToken, oldRefreshJTI, err := authinfra.GenerateRefreshToken(
		userID,
		"refresh-secret",
		24*time.Hour,
	)
	if err != nil {
		t.Fatalf("generate old refresh token: %v", err)
	}

	var gotRotateUserID uuid.UUID
	var gotOldJTI string
	var gotNewJTI string
	var gotAccessJTI string

	svc := &Service{
		Repo: &repoStub{
			createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
				t.Fatal("Create should not be called in Refresh")
				return uuid.Nil, nil
			},
			getByLoginFn: func(_ context.Context, _ string) (*UserWithPassword, error) {
				t.Fatal("GetByLogin should not be called in Refresh")
				return nil, nil
			},
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*User, error) {
				t.Fatal("GetByID should not be called in Refresh")
				return nil, nil
			},
		},
		AccessTokens: &accessStoreStub{
			storeFn: func(_ context.Context, jti string, gotUserID uuid.UUID, ttl time.Duration) error {
				if gotUserID != userID {
					t.Fatalf("unexpected access user id: %s", gotUserID)
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
		},
		RefreshTokens: &refreshStoreStub{
			storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
				t.Fatal("Store should not be called in Refresh")
				return nil
			},
			rotateFn: func(
				_ context.Context,
				gotUserID uuid.UUID,
				oldRefreshJTI, newRefreshJTI string,
				ttl time.Duration,
			) error {
				gotRotateUserID = gotUserID
				gotOldJTI = oldRefreshJTI
				gotNewJTI = newRefreshJTI
				if ttl != 24*time.Hour {
					t.Fatalf("unexpected refresh ttl: %v", ttl)
				}
				return nil
			},
		},
		JWTSecret:       "refresh-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}

	tokens, err := svc.Refresh(context.Background(), oldRefreshToken)
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if gotRotateUserID != userID {
		t.Fatalf("unexpected rotate user id: got %s want %s", gotRotateUserID, userID)
	}
	if gotOldJTI != oldRefreshJTI {
		t.Fatalf("old refresh jti mismatch: got %s want %s", gotOldJTI, oldRefreshJTI)
	}
	refreshClaims, err := authinfra.ParseRefreshTokenClaims(tokens.RefreshToken, "refresh-secret")
	if err != nil {
		t.Fatalf("parse refresh token claims: %v", err)
	}
	if gotNewJTI != refreshClaims.JTI {
		t.Fatalf("new refresh jti mismatch: got %s want %s", gotNewJTI, refreshClaims.JTI)
	}
	if refreshClaims.UserID != userID {
		t.Fatalf("unexpected refresh token subject: got %s want %s", refreshClaims.UserID, userID)
	}
	claims, err := authinfra.ParseTokenClaims(tokens.AccessToken, "refresh-secret")
	if err != nil {
		t.Fatalf("parse access token claims: %v", err)
	}
	if gotAccessJTI != claims.JTI {
		t.Fatalf("access jti mismatch: got %s want %s", gotAccessJTI, claims.JTI)
	}

	if claims.UserID != userID {
		t.Fatalf("unexpected token subject: got %s want %s", claims.UserID, userID)
	}
}

func TestServiceRefreshInvalidToken(t *testing.T) {
	svc := &Service{
		Repo: &repoStub{
			createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
				t.Fatal("Create should not be called in Refresh")
				return uuid.Nil, nil
			},
			getByLoginFn: func(_ context.Context, _ string) (*UserWithPassword, error) {
				t.Fatal("GetByLogin should not be called in Refresh")
				return nil, nil
			},
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*User, error) {
				t.Fatal("GetByID should not be called in Refresh")
				return nil, nil
			},
		},
		RefreshTokens: &refreshStoreStub{
			storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
				t.Fatal("Store should not be called in Refresh")
				return nil
			},
			rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
				t.Fatal("Rotate should not be called for invalid refresh JWT")
				return nil
			},
		},
		JWTSecret:       "refresh-secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: 24 * time.Hour,
	}

	_, err := svc.Refresh(context.Background(), "bad-refresh")
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}
}

func TestServiceLogout(t *testing.T) {
	userID := uuid.New()
	refreshToken, refreshJTI, err := authinfra.GenerateRefreshToken(userID, "logout-secret", 24*time.Hour)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}
	const accessJTI = "9f918be4-c7fc-4a46-b00d-7b2f3fcfc840"
	var gotRevokedRefreshUserID uuid.UUID
	var gotRevokedRefreshJTI string
	var gotRevokedAccessJTI string

	svc := &Service{
		Repo: &repoStub{
			createFn: func(_ context.Context, _, _, _, _ string, _ time.Time) (uuid.UUID, error) {
				t.Fatal("Create should not be called in Logout")
				return uuid.Nil, nil
			},
			getByLoginFn: func(_ context.Context, _ string) (*UserWithPassword, error) {
				t.Fatal("GetByLogin should not be called in Logout")
				return nil, nil
			},
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*User, error) {
				t.Fatal("GetByID should not be called in Logout")
				return nil, nil
			},
		},
		AccessTokens: &accessStoreStub{
			storeFn: func(_ context.Context, _ string, _ uuid.UUID, _ time.Duration) error {
				t.Fatal("StoreAccessToken should not be called in Logout")
				return nil
			},
			isActiveFn: func(_ context.Context, _ string, _ uuid.UUID) (bool, error) {
				t.Fatal("IsAccessTokenActive should not be called in Logout")
				return false, nil
			},
			revokeFn: func(_ context.Context, jti string) error {
				gotRevokedAccessJTI = jti
				return nil
			},
		},
		RefreshTokens: &refreshStoreStub{
			storeFn: func(_ context.Context, _ uuid.UUID, _ string, _ time.Duration) error {
				t.Fatal("Store should not be called in Logout")
				return nil
			},
			rotateFn: func(_ context.Context, _ uuid.UUID, _, _ string, _ time.Duration) error {
				t.Fatal("Rotate should not be called in Logout")
				return nil
			},
			revokeFn: func(_ context.Context, gotUserID uuid.UUID, gotRefreshJTI string) error {
				gotRevokedRefreshUserID = gotUserID
				gotRevokedRefreshJTI = gotRefreshJTI
				return nil
			},
		},
		JWTSecret: "logout-secret",
	}

	if err := svc.Logout(context.Background(), refreshToken, accessJTI); err != nil {
		t.Fatalf("logout failed: %v", err)
	}
	if gotRevokedRefreshUserID != userID {
		t.Fatalf("unexpected revoked refresh user id: got %s want %s", gotRevokedRefreshUserID, userID)
	}
	if gotRevokedRefreshJTI != refreshJTI {
		t.Fatalf("unexpected revoked refresh jti: got %s want %s", gotRevokedRefreshJTI, refreshJTI)
	}
	if gotRevokedAccessJTI != accessJTI {
		t.Fatalf("unexpected revoked access jti: %s", gotRevokedAccessJTI)
	}
}
