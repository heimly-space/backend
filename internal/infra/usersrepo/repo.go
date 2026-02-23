package usersrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "heimly.space/backend/internal/domain/users"
)

type Repo struct {
	DB *pgxpool.Pool
}

var _ domain.Repository = (*Repo)(nil)

func (r *Repo) Create(
	ctx context.Context,
	login, email, name, hash string,
	birthday time.Time,
) (uuid.UUID, error) {
	const q = `
		INSERT INTO users (login, email, name, hashed_password, birthday)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	var id uuid.UUID
	err := r.DB.QueryRow(
		ctx,
		q,
		login,
		email,
		name,
		hash,
		nullableDate(birthday),
	).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return uuid.Nil, domain.ErrUserExists
		}
		return uuid.Nil, err
	}

	return id, nil
}

func (r *Repo) GetByLogin(ctx context.Context, login string) (*domain.UserWithPassword, error) {
	const q = `
		SELECT id, login, email, name, hashed_password, birthday
		FROM users
		WHERE login = $1
	`

	u := &domain.UserWithPassword{}
	var birthday sql.NullTime

	err := r.DB.QueryRow(ctx, q, login).Scan(
		&u.ID,
		&u.Login,
		&u.Email,
		&u.Name,
		&u.HashedPassword,
		&birthday,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	u.Birthday = nullDate(birthday)
	return u, nil
}

func (r *Repo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const q = `
		SELECT id, login, email, name, birthday
		FROM users
		WHERE id = $1
	`

	u := &domain.User{}
	var birthday sql.NullTime
	err := r.DB.QueryRow(ctx, q, id).Scan(
		&u.ID,
		&u.Login,
		&u.Email,
		&u.Name,
		&birthday,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	u.Birthday = nullDate(birthday)
	return u, nil
}

func nullableDate(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func nullDate(v sql.NullTime) time.Time {
	if !v.Valid {
		return time.Time{}
	}
	return v.Time
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == pgerrcode.UniqueViolation
}
