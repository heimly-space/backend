package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"heimly.space/backend/internal/cfg"
	httpmw "heimly.space/backend/internal/transport/http/middleware"
	"heimly.space/backend/internal/transport/http/users"
)

func NewRouter(authHandlers *users.AuthHandlers, cfg *cfg.Config) chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandlers.Register)
			r.Post("/login", authHandlers.Login)
			r.Post("/refresh", authHandlers.Refresh)
			r.Post("/logout", authHandlers.Logout)
		})

		r.Route("/users", func(r chi.Router) {
			r.Use(httpmw.JWTMiddleware(cfg.JWTSecret, authHandlers.Users.AccessTokens))
			r.Get("/me", authHandlers.GetMe)
		})
	})

	return r
}
