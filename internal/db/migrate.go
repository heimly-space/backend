package db

import (
	"errors"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

func RunMigrations(pool *pgxpool.Pool) {
	sqlDB := stdlib.OpenDBFromPool(pool)

	driver, err := pgx.WithInstance(sqlDB, &pgx.Config{})
	if err != nil {
		log.Fatal("failed to create migration driver:", err)
	}

	d, err := iofs.New(MigrationsFS, "migrations")
	if err != nil {
		log.Fatal("failed to create iofs source:", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs",
		d,
		"postgres",
		driver,
	)
	if err != nil {
		log.Fatal("failed to init migrate:", err)
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatal("migration failed:", err)
	}

	fmt.Println("✅ Database migrations applied")
}
