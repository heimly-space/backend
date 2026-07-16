package tasks

import "errors"

var (
	ErrTaskNotFound           = errors.New("task not found")
	ErrTaskTitleRequired      = errors.New("task title is required")
	ErrTaskStatusInvalid      = errors.New("invalid task status")
	ErrTaskPatchEmpty         = errors.New("empty task patch")
	ErrHouseholdNotFound      = errors.New("household not found")
	ErrForbidden              = errors.New("forbidden")
	ErrAssigneeNotInHousehold = errors.New("assignee is not household member")
	ErrInvalidCursor          = errors.New("invalid cursor")
)
