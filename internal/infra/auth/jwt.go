package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var ErrInvalidToken = errors.New("invalid token")

func GenerateToken(userID uuid.UUID, secret string, ttl time.Duration) (string, error) {
	token, _, err := GenerateTokenWithJTI(userID, secret, ttl)
	return token, err
}

func GenerateTokenWithJTI(userID uuid.UUID, secret string, ttl time.Duration) (string, string, error) {
	now := time.Now()
	jti := uuid.NewString()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		ID:        jti,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", "", err
	}
	return tokenStr, jti, nil
}

func ParseToken(tokenStr, secret string) (uuid.UUID, error) {
	claims, err := ParseTokenClaims(tokenStr, secret)
	if err != nil {
		return uuid.Nil, err
	}
	return claims.UserID, nil
}

type TokenClaims struct {
	UserID uuid.UUID
	JTI    string
}

func ParseTokenClaims(tokenStr, secret string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&jwt.RegisteredClaims{},
		func(t *jwt.Token) (interface{}, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return []byte(secret), nil
		},
	)
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*jwt.RegisteredClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("%w: subject is not a valid uuid", ErrInvalidToken)
	}
	if claims.ID == "" {
		return nil, fmt.Errorf("%w: missing jti", ErrInvalidToken)
	}
	if _, err := uuid.Parse(claims.ID); err != nil {
		return nil, fmt.Errorf("%w: jti is not a valid uuid", ErrInvalidToken)
	}

	return &TokenClaims{
		UserID: userID,
		JTI:    claims.ID,
	}, nil
}
