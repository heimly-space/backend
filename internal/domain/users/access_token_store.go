package users

import (
	"context"
	"time"
	"uuid"
)

type AccessTokenStore interface {
	StoreAccessToken(ctx context.Context, jti string, userID uuid.UUID, ttl time.Duration) error
	IsAccessTokenActive(ctx context.Context, jti string, userID uuid.UUID) (bool, error)
	RevokeAccessToken(ctx context.Context, jti string) error
}
