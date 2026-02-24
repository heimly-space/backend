package householdsrepo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "heimly.space/backend/internal/domain/households"
)

type Repo struct {
	DB *pgxpool.Pool
}

var _ domain.Repository = (*Repo)(nil)

func (r *Repo) Create(ctx context.Context, name string, ownerID uuid.UUID) (*domain.Household, error) {
	const createHouseholdQ = `
		INSERT INTO households (name)
		VALUES ($1)
		RETURNING id, name, created_at
	`
	const addOwnerQ = `
		INSERT INTO household_members (household_id, user_id, role)
		VALUES ($1, $2, 'owner')
	`

	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	h := &domain.Household{}
	if err := tx.QueryRow(ctx, createHouseholdQ, name).Scan(&h.ID, &h.Name, &h.CreatedAt); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, addOwnerQ, h.ID, ownerID); err != nil {
		if isForeignKeyViolation(err) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return h, nil
}

func (r *Repo) Exists(ctx context.Context, householdID uuid.UUID) (bool, error) {
	const q = `
		SELECT EXISTS(
			SELECT 1
			FROM households
			WHERE id = $1
		)
	`
	var exists bool
	if err := r.DB.QueryRow(ctx, q, householdID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (r *Repo) IsMember(ctx context.Context, householdID, userID uuid.UUID) (bool, error) {
	const q = `
		SELECT EXISTS(
			SELECT 1
			FROM household_members
			WHERE household_id = $1 AND user_id = $2
		)
	`
	var isMember bool
	if err := r.DB.QueryRow(ctx, q, householdID, userID).Scan(&isMember); err != nil {
		return false, err
	}
	return isMember, nil
}

func (r *Repo) AddMemberByEmail(
	ctx context.Context,
	householdID uuid.UUID,
	email string,
) (*domain.Member, error) {
	const userLookupQ = `
		SELECT id, email, name
		FROM users
		WHERE lower(email) = lower($1)
	`
	const addMemberQ = `
		INSERT INTO household_members (household_id, user_id, role)
		VALUES ($1, $2, 'member')
		RETURNING role, created_at
	`

	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	member := &domain.Member{}
	if err := tx.QueryRow(ctx, userLookupQ, email).Scan(&member.UserID, &member.Email, &member.Name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	if err := tx.QueryRow(ctx, addMemberQ, householdID, member.UserID).Scan(&member.Role, &member.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrMemberAlreadyExists
		}
		if isForeignKeyViolation(err) {
			return nil, domain.ErrHouseholdNotFound
		}
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return member, nil
}

func (r *Repo) ListMembers(ctx context.Context, householdID uuid.UUID) ([]domain.Member, error) {
	const q = `
		SELECT hm.user_id, u.email, u.name, hm.role, hm.created_at
		FROM household_members hm
		JOIN users u ON u.id = hm.user_id
		WHERE hm.household_id = $1
		ORDER BY hm.created_at ASC
	`

	rows, err := r.DB.Query(ctx, q, householdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.Member, 0)
	for rows.Next() {
		var m domain.Member
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Repo) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	cursor *domain.ListCursor,
	limit int,
) ([]domain.HouseholdWithRole, error) {
	const q = `
		SELECT h.id, h.name, hm.role, h.created_at, hm.created_at
		FROM household_members hm
		JOIN households h ON h.id = hm.household_id
		WHERE hm.user_id = $1
		  AND (
		    $2::timestamptz IS NULL
		    OR hm.created_at < $2
		    OR (hm.created_at = $2 AND hm.household_id < $3)
		  )
		ORDER BY hm.created_at DESC, hm.household_id DESC
		LIMIT $4
	`

	var cursorCreatedAt any
	var cursorHouseholdID any
	if cursor != nil {
		cursorCreatedAt = cursor.MemberCreatedAt
		cursorHouseholdID = cursor.HouseholdID
	}

	rows, err := r.DB.Query(ctx, q, userID, cursorCreatedAt, cursorHouseholdID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]domain.HouseholdWithRole, 0)
	for rows.Next() {
		var item domain.HouseholdWithRole
		if err := rows.Scan(
			&item.ID,
			&item.Name,
			&item.Role,
			&item.CreatedAt,
			&item.MemberCreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == pgerrcode.UniqueViolation
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == pgerrcode.ForeignKeyViolation
}
