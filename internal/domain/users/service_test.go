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

func TestServiceRegister(t *testing.T) {
	userID := uuid.New()
	birthday := time.Date(1995, time.October, 15, 0, 0, 0, 0, time.UTC)
	var gotHash string

	svc := &Service{
		Repo: &repoStub{
			createFn: func(_ context.Context, login, email, name, hash string, gotBirthday time.Time) (uuid.UUID, error) {
				if login != "john" || email != "john@example.com" || name != "John Doe" {
					t.Fatalf("unexpected create params: %q %q %q", login, email, name)
				}
				if !gotBirthday.Equal(birthday) {
					t.Fatalf("unexpected birthday: %v", gotBirthday)
				}
				gotHash = hash
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
		JWTSecret: "register-secret",
		JWTExpiry: time.Hour,
	}

	token, err := svc.Register(context.Background(), "john", "john@example.com", "John Doe", "secret-pass", birthday)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	if gotHash == "" || gotHash == "secret-pass" {
		t.Fatal("expected password hash to be created")
	}
	if err := authinfra.CheckPassword(gotHash, "secret-pass"); err != nil {
		t.Fatalf("hash does not match source password: %v", err)
	}

	parsedID, err := authinfra.ParseToken(token, "register-secret")
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if parsedID != userID {
		t.Fatalf("unexpected token subject: got %s want %s", parsedID, userID)
	}
}

func TestServiceLoginSuccess(t *testing.T) {
	userID := uuid.New()
	hash, err := authinfra.HashPassword("secret-pass")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

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
					User: User{
						ID:    userID,
						Login: "john",
					},
					HashedPassword: hash,
				}, nil
			},
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*User, error) {
				t.Fatal("GetByID should not be called in Login")
				return nil, nil
			},
		},
		JWTSecret: "login-secret",
		JWTExpiry: time.Hour,
	}

	token, err := svc.Login(context.Background(), "john", "secret-pass")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	parsedID, err := authinfra.ParseToken(token, "login-secret")
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if parsedID != userID {
		t.Fatalf("unexpected token subject: got %s want %s", parsedID, userID)
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
			JWTSecret: "login-secret",
			JWTExpiry: time.Hour,
		}

		_, err := svc.Login(context.Background(), "john", "secret-pass")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("wrong-password", func(t *testing.T) {
		hash, err := authinfra.HashPassword("right-pass")
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
						HashedPassword: hash,
					}, nil
				},
				getByIDFn: func(_ context.Context, _ uuid.UUID) (*User, error) {
					t.Fatal("GetByID should not be called in Login")
					return nil, nil
				},
			},
			JWTSecret: "login-secret",
			JWTExpiry: time.Hour,
		}

		_, err = svc.Login(context.Background(), "john", "wrong-pass")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}
