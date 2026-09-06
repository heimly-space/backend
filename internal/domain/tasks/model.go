package tasks

import (
	"time"

	"github.com/google/uuid"
)

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
)

type Task struct {
	ID          uuid.UUID
	HouseholdID uuid.UUID
	Title       string
	Description string
	Status      string
	DueAt       time.Time
	AssigneeIDs []uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TaskListCursor struct {
	CreatedAt time.Time
	TaskID    uuid.UUID
}

type CreateTaskInput struct {
	Title       string
	Description string
	Status      string
	DueAt       time.Time
	AssigneeIDs []uuid.UUID
}

type ListFilter struct {
	Status     string
	AssigneeID *uuid.UUID
}

type PatchTaskInput struct {
	Title        *string
	Description  *string
	Status       *string
	DueAt        *time.Time
	DueAtSet     bool
	AssigneeIDs  []uuid.UUID
	AssigneesSet bool
}

type ListResult struct {
	Items      []Task
	NextCursor string
}
