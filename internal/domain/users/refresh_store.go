package users

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type RefreshTokenStore interface {
	Store(ctx context.Context, userID uuid.UUID, refreshJTI string, ttl time.Duration) error
	Rotate(ctx context.Context, userID uuid.UUID, oldRefreshJTI, newRefreshJTI string, ttl time.Duration) error
	Revoke(ctx context.Context, userID uuid.UUID, refreshJTI string) error
}
