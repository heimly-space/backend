package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"heimly.space/backend/internal/cfg"
	db "heimly.space/backend/internal/db"
	"heimly.space/backend/internal/domain/users"
	"heimly.space/backend/internal/infra/usersrepo"
	httpx "heimly.space/backend/internal/transport/http"
	usershttp "heimly.space/backend/internal/transport/http/users"
)

type App struct {
	HTTPServer *http.Server
	DB         *pgxpool.Pool
}

func New(cfg *cfg.Config) *App {
	pool := db.ConnectDB(cfg)
	db.RunMigrations(pool)

	userRepo := &usersrepo.Repo{DB: pool}
	userService := &users.Service{
		Repo:      userRepo,
		JWTSecret: cfg.JWTSecret,
		JWTExpiry: 24 * time.Hour,
	}
	authHandlers := &usershttp.AuthHandlers{Users: userService}
	router := httpx.NewRouter(authHandlers, cfg)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &App{
		HTTPServer: srv,
		DB:         pool,
	}
}
