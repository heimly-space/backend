package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestGenerateRefreshToken(t *testing.T) {
	userID := uuid.New()
	token, jti, err := GenerateRefreshToken(userID, "refresh-secret", time.Hour)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if jti == "" {
		t.Fatal("expected non-empty token jti")
	}
	claims, err := ParseRefreshTokenClaims(token, "refresh-secret")
	if err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}
	if claims.UserID != userID {
		t.Fatalf("unexpected refresh token user id: got %s want %s", claims.UserID, userID)
	}
	if claims.JTI != jti {
		t.Fatalf("unexpected refresh token jti: got %s want %s", claims.JTI, jti)
	}
}

func TestParseRefreshTokenWrongSecret(t *testing.T) {
	token, _, err := GenerateRefreshToken(uuid.New(), "secret-a", time.Hour)
	if err != nil {
		t.Fatalf("generate refresh token: %v", err)
	}

	_, err = ParseRefreshTokenClaims(token, "secret-b")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}
