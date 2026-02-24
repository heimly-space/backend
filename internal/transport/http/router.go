package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"heimly.space/backend/internal/cfg"
	householdhttp "heimly.space/backend/internal/transport/http/households"
	httpmw "heimly.space/backend/internal/transport/http/middleware"
	"heimly.space/backend/internal/transport/http/users"
)

func NewRouter(
	authHandlers *users.AuthHandlers,
	householdHandlers *householdhttp.Handlers,
	cfg *cfg.Config,
) chi.Router {
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

		r.Group(func(r chi.Router) {
			r.Use(httpmw.JWTMiddleware(cfg.JWTSecret, authHandlers.Users.AccessTokens))

			r.Route("/users", func(r chi.Router) {
				r.Get("/me", authHandlers.GetMe)
			})

			r.Route("/households", func(r chi.Router) {
				r.Post("/", householdHandlers.Create)
				r.Post("/{id}/members", householdHandlers.InviteMember)
				r.Get("/{id}/members", householdHandlers.ListMembers)
			})
		})

	})

	return r
}
