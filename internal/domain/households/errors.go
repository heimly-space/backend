package households

import "errors"

var (
	ErrHouseholdNameRequired = errors.New("household name is required")
	ErrHouseholdNotFound     = errors.New("household not found")
	ErrForbidden             = errors.New("forbidden")
	ErrUserNotFound          = errors.New("user not found")
	ErrMemberAlreadyExists   = errors.New("member already exists")
	ErrInvalidCursor         = errors.New("invalid cursor")
)
