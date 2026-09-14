package auth

import (
	"time"
	"uuid"
)

func GenerateRefreshToken(userID uuid.UUID, secret string, ttl time.Duration) (string, string, error) {
	return GenerateTokenWithJTI(userID, secret, ttl)
}

func ParseRefreshTokenClaims(tokenStr, secret string) (*TokenClaims, error) {
	return ParseTokenClaims(tokenStr, secret)
}
