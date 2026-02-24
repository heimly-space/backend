package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	authinfra "heimly.space/backend/internal/infra/auth"
)

type ctxKey string

const userIDKey ctxKey = "user_id"

type AccessTokenChecker interface {
	IsAccessTokenActive(ctx context.Context, jti string, userID uuid.UUID) (bool, error)
}

func JWTMiddleware(secret string, checker AccessTokenChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, "missing token", http.StatusUnauthorized)
				return
			}

			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			claims, err := authinfra.ParseTokenClaims(token, secret)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			if checker == nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			isActive, err := checker.IsAccessTokenActive(r.Context(), claims.JTI, claims.UserID)
			if err != nil {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if !isActive {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

func ContextWithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
