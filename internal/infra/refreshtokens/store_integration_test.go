package refreshtokens

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	domain "heimly.space/backend/internal/domain/users"
)

func TestStoreRotateAndRevokeIntegration(t *testing.T) {
	store := newIntegrationStore(t)
	ctx := context.Background()
	userID := uuid.New()
	oldJTI := uuid.NewString()
	newJTI := uuid.NewString()
	nextJTI := uuid.NewString()

	if err := store.Store(ctx, userID, oldJTI, 2*time.Minute); err != nil {
		t.Fatalf("store refresh jti: %v", err)
	}
	if err := store.Rotate(ctx, userID, oldJTI, newJTI, 2*time.Minute); err != nil {
		t.Fatalf("rotate refresh jti: %v", err)
	}

	err := store.Rotate(ctx, userID, oldJTI, nextJTI, 2*time.Minute)
	if !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken for old jti, got %v", err)
	}

	if err := store.Revoke(ctx, userID, newJTI); err != nil {
		t.Fatalf("revoke refresh jti: %v", err)
	}
	err = store.Rotate(ctx, userID, newJTI, nextJTI, 2*time.Minute)
	if !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken for revoked jti, got %v", err)
	}
}

func TestStoreRejectsCrossUserRotationAndRevokeIntegration(t *testing.T) {
	store := newIntegrationStore(t)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()
	jti := uuid.NewString()
	rotatedJTI := uuid.NewString()

	if err := store.Store(ctx, userA, jti, 2*time.Minute); err != nil {
		t.Fatalf("store refresh jti: %v", err)
	}

	err := store.Rotate(ctx, userB, jti, rotatedJTI, 2*time.Minute)
	if !errors.Is(err, domain.ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken for cross-user rotate, got %v", err)
	}

	if err := store.Revoke(ctx, userB, jti); err != nil {
		t.Fatalf("cross-user revoke should be safe no-op, got %v", err)
	}

	if err := store.Rotate(ctx, userA, jti, rotatedJTI, 2*time.Minute); err != nil {
		t.Fatalf("expected original user session to remain active, got %v", err)
	}
}

func newIntegrationStore(t *testing.T) *Store {
	t.Helper()

	cacheURL := os.Getenv("VALKEY_TEST_URL")
	if cacheURL == "" {
		cacheURL = "redis://localhost:6379/0"
	}

	store, err := NewStoreFromURL(cacheURL)
	if err != nil {
		t.Skipf("skip integration test: valkey not available at %s (%v)", cacheURL, err)
	}

	store.keyPrefix = fmt.Sprintf("auth:test:refresh:%s:", uuid.NewString())
	return store
}
