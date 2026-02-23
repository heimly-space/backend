package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestGenerateAndParseToken(t *testing.T) {
	userID := uuid.New()
	secret := "test-secret"

	token, err := GenerateToken(userID, secret, time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	gotID, err := ParseToken(token, secret)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if gotID != userID {
		t.Fatalf("unexpected user id: got %s want %s", gotID, userID)
	}
}

func TestParseTokenWrongSecret(t *testing.T) {
	token, err := GenerateToken(uuid.New(), "secret-a", time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	_, err = ParseToken(token, "secret-b")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestParseTokenExpired(t *testing.T) {
	token, err := GenerateToken(uuid.New(), "secret", -time.Second)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	_, err = ParseToken(token, "secret")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestParseTokenInvalidSubjectUUID(t *testing.T) {
	claims := jwt.RegisteredClaims{
		Subject:   "not-a-uuid",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = ParseToken(token, "secret")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestParseTokenWrongSigningMethod(t *testing.T) {
	claims := jwt.RegisteredClaims{
		Subject:   uuid.NewString(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	_, err = ParseToken(token, "secret")
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}
