package tasksrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	domain "heimly.space/backend/internal/domain/tasks"
)

type Repo struct {
	DB *pgxpool.Pool
}

var _ domain.Repository = (*Repo)(nil)

func (r *Repo) HouseholdExists(ctx context.Context, householdID uuid.UUID) (bool, error) {
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

func (r *Repo) IsHouseholdMember(ctx context.Context, householdID, userID uuid.UUID) (bool, error) {
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

func (r *Repo) Create(ctx context.Context, task *domain.Task) (*domain.Task, error) {
	const createTaskQ = `
		INSERT INTO tasks (household_id, title, description, status, due_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(
		ctx,
		createTaskQ,
		task.HouseholdID,
		task.Title,
		nullableText(task.Description),
		task.Status,
		nullableDate(task.DueAt),
	).Scan(&task.ID); err != nil {
		if isForeignKeyViolation(err) {
			return nil, domain.ErrHouseholdNotFound
		}
		return nil, err
	}

	if err := replaceAssignees(ctx, tx, task.ID, task.AssigneeIDs); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, task.ID)
}

func (r *Repo) ListByHousehold(
	ctx context.Context,
	householdID uuid.UUID,
	filter domain.ListFilter,
	cursor *domain.TaskListCursor,
	limit int,
) ([]domain.Task, error) {
	const q = `
		SELECT
			t.id,
			t.household_id,
			t.title,
			t.description,
			t.status,
			t.due_at,
			t.created_at,
			t.updated_at,
			COALESCE(
				array_agg(ta.user_id::text ORDER BY ta.assigned_at DESC)
				FILTER (WHERE ta.user_id IS NOT NULL),
				'{}'
			) AS assignee_ids
		FROM tasks t
		LEFT JOIN task_assignees ta ON ta.task_id = t.id
		WHERE t.household_id = $1
		  AND ($2::text IS NULL OR t.status = $2)
		  AND (
			$3::uuid IS NULL
			OR EXISTS (
				SELECT 1
				FROM task_assignees ta_filter
				WHERE ta_filter.task_id = t.id AND ta_filter.user_id = $3
			)
		  )
		  AND (
			$4::timestamptz IS NULL
			OR t.created_at < $4
			OR (t.created_at = $4 AND t.id < $5)
		  )
		GROUP BY t.id
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT $6
	`

	statusArg := nullableText(filter.Status)
	var assigneeArg any
	if filter.AssigneeID != nil {
		assigneeArg = *filter.AssigneeID
	}

	var cursorCreatedAt any
	var cursorTaskID any
	if cursor != nil {
		cursorCreatedAt = cursor.CreatedAt
		cursorTaskID = cursor.TaskID
	}

	rows, err := r.DB.Query(ctx, q, householdID, statusArg, assigneeArg, cursorCreatedAt, cursorTaskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resultCap := limit
	if resultCap < 0 {
		resultCap = 0
	}
	result := make([]domain.Task, 0, resultCap)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Repo) GetByID(ctx context.Context, taskID uuid.UUID) (*domain.Task, error) {
	const q = `
		SELECT
			t.id,
			t.household_id,
			t.title,
			t.description,
			t.status,
			t.due_at,
			t.created_at,
			t.updated_at,
			COALESCE(
				array_agg(ta.user_id::text ORDER BY ta.assigned_at DESC)
				FILTER (WHERE ta.user_id IS NOT NULL),
				'{}'
			) AS assignee_ids
		FROM tasks t
		LEFT JOIN task_assignees ta ON ta.task_id = t.id
		WHERE t.id = $1
		GROUP BY t.id
	`

	task, err := scanTask(r.DB.QueryRow(ctx, q, taskID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return task, nil
}

func (r *Repo) Update(ctx context.Context, task *domain.Task, assigneesChanged bool) (*domain.Task, error) {
	const updateTaskQ = `
		UPDATE tasks
		SET title = $2,
			description = $3,
			status = $4,
			due_at = $5,
			updated_at = now()
		WHERE id = $1
	`

	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	res, err := tx.Exec(
		ctx,
		updateTaskQ,
		task.ID,
		task.Title,
		nullableText(task.Description),
		task.Status,
		nullableDate(task.DueAt),
	)
	if err != nil {
		return nil, err
	}
	if res.RowsAffected() == 0 {
		return nil, nil
	}

	if assigneesChanged {
		if err := replaceAssignees(ctx, tx, task.ID, task.AssigneeIDs); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, task.ID)
}

func (r *Repo) Delete(ctx context.Context, taskID uuid.UUID) (bool, error) {
	const q = `
		DELETE FROM tasks
		WHERE id = $1
	`

	res, err := r.DB.Exec(ctx, q, taskID)
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanTask(s scanner) (*domain.Task, error) {
	var task domain.Task
	var description sql.NullString
	var dueAt sql.NullTime
	var assigneeIDs []string

	if err := s.Scan(
		&task.ID,
		&task.HouseholdID,
		&task.Title,
		&description,
		&task.Status,
		&dueAt,
		&task.CreatedAt,
		&task.UpdatedAt,
		&assigneeIDs,
	); err != nil {
		return nil, err
	}

	if description.Valid {
		task.Description = description.String
	}
	if dueAt.Valid {
		task.DueAt = dueAt.Time
	}
	if len(assigneeIDs) > 0 {
		task.AssigneeIDs = make([]uuid.UUID, 0, len(assigneeIDs))
		for _, assignee := range assigneeIDs {
			parsed, err := uuid.Parse(assignee)
			if err != nil {
				return nil, err
			}
			task.AssigneeIDs = append(task.AssigneeIDs, parsed)
		}
	}

	return &task, nil
}

func replaceAssignees(ctx context.Context, tx pgx.Tx, taskID uuid.UUID, assigneeIDs []uuid.UUID) error {
	const clearAssigneesQ = `
		DELETE FROM task_assignees
		WHERE task_id = $1
	`
	const addAssigneeQ = `
		INSERT INTO task_assignees (task_id, user_id)
		SELECT t.id, $2
		FROM tasks t
		JOIN household_members hm ON hm.household_id = t.household_id AND hm.user_id = $2
		WHERE t.id = $1
	`

	if _, err := tx.Exec(ctx, clearAssigneesQ, taskID); err != nil {
		return err
	}

	uniqueIDs := uniqueAssigneeIDs(assigneeIDs)
	if len(uniqueIDs) == 0 {
		return nil
	}

	for _, assigneeID := range uniqueIDs {
		res, err := tx.Exec(ctx, addAssigneeQ, taskID, assigneeID)
		if err != nil {
			if isForeignKeyViolation(err) {
				return domain.ErrAssigneeNotInHousehold
			}
			return err
		}
		if res.RowsAffected() != 1 {
			return domain.ErrAssigneeNotInHousehold
		}
	}

	return nil
}

func uniqueAssigneeIDs(assigneeIDs []uuid.UUID) []uuid.UUID {
	if len(assigneeIDs) == 0 {
		return nil
	}

	seen := make(map[uuid.UUID]struct{}, len(assigneeIDs))
	result := make([]uuid.UUID, 0, len(assigneeIDs))
	for _, assigneeID := range assigneeIDs {
		if assigneeID == uuid.Nil {
			continue
		}
		if _, ok := seen[assigneeID]; ok {
			continue
		}
		seen[assigneeID] = struct{}{}
		result = append(result, assigneeID)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableDate(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == pgerrcode.ForeignKeyViolation
}
