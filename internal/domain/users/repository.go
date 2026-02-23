package users

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, login, email, name, hash string, birthday time.Time) (uuid.UUID, error)
	GetByLogin(ctx context.Context, login string) (*UserWithPassword, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
}
