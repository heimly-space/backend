package tasks

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	HouseholdExists(ctx context.Context, householdID uuid.UUID) (bool, error)
	IsHouseholdMember(ctx context.Context, householdID, userID uuid.UUID) (bool, error)
	Create(ctx context.Context, task *Task) (*Task, error)
	ListByHousehold(
		ctx context.Context,
		householdID uuid.UUID,
		filter ListFilter,
		cursor *TaskListCursor,
		limit int,
	) ([]Task, error)
	GetByID(ctx context.Context, taskID uuid.UUID) (*Task, error)
	Update(ctx context.Context, task *Task, assigneesChanged bool) (*Task, error)
	Delete(ctx context.Context, taskID uuid.UUID) (bool, error)
}
