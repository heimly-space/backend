package app

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"heimly.space/backend/internal/cfg"
	db "heimly.space/backend/internal/db"
	"heimly.space/backend/internal/domain/households"
	"heimly.space/backend/internal/domain/users"
	"heimly.space/backend/internal/infra/householdsrepo"
	"heimly.space/backend/internal/infra/refreshtokens"
	"heimly.space/backend/internal/infra/usersrepo"
	httpx "heimly.space/backend/internal/transport/http"
	householdshttp "heimly.space/backend/internal/transport/http/households"
	usershttp "heimly.space/backend/internal/transport/http/users"
)

type App struct {
	HTTPServer *http.Server
	DB         *pgxpool.Pool
}

func New(cfg *cfg.Config) *App {
	pool := db.ConnectDB(cfg)
	db.RunMigrations(pool)
	refreshStore, err := refreshtokens.NewStoreFromURL(cfg.CacheURL)
	if err != nil {
		panic(err)
	}

	householdRepo := &householdsrepo.Repo{DB: pool}
	householdService := &households.Service{
		Repo:         householdRepo,
		CursorSecret: cfg.JWTSecret,
	}

	userRepo := &usersrepo.Repo{DB: pool}
	userService := &users.Service{
		Repo:            userRepo,
		AccessTokens:    refreshStore,
		RefreshTokens:   refreshStore,
		JWTSecret:       cfg.JWTSecret,
		AccessTokenTTL:  cfg.AccessTokenTTL,
		RefreshTokenTTL: cfg.RefreshTokenTTL,
	}
	authHandlers := &usershttp.AuthHandlers{Users: userService}
	householdHandlers := &householdshttp.Handlers{Households: householdService}
	router := httpx.NewRouter(authHandlers, householdHandlers, cfg)

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
