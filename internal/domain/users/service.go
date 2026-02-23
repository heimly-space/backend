package users

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	authinfra "heimly.space/backend/internal/infra/auth"
)

type Service struct {
	Repo      Repository
	JWTSecret string
	JWTExpiry time.Duration
}

func (s *Service) Register(
	ctx context.Context,
	login, email, name, password string,
	birthday time.Time,
) (string, error) {
	hash, err := authinfra.HashPassword(password)
	if err != nil {
		return "", err
	}

	userID, err := s.Repo.Create(ctx, login, email, name, hash, birthday)
	if err != nil {
		return "", err
	}

	return authinfra.GenerateToken(userID, s.JWTSecret, s.JWTExpiry)
}

func (s *Service) Login(ctx context.Context, login, password string) (string, error) {
	user, err := s.Repo.GetByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	if err := authinfra.CheckPassword(user.HashedPassword, password); err != nil {
		return "", ErrInvalidCredentials
	}

	return authinfra.GenerateToken(user.ID, s.JWTSecret, s.JWTExpiry)
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	return s.Repo.GetByID(ctx, id)
}
