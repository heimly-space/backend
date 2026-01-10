package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"heimly.space/backend/internal/cfg"
)

func ConnectDB(cfg *cfg.Config) *pgxpool.Pool {
	config, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Unable to parse DATABASE_URL:", err)
	}

	config.MaxConns = 20
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		log.Fatal("Unable to create connection pool:", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal("Cannot ping database:", err)
	}

	fmt.Println("Connected to Heimly DB!")
	return pool
}
