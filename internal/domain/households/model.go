package households

import (
	"time"

	"github.com/google/uuid"
)

type Household struct {
	ID        uuid.UUID
	Name      string
	CreatedAt time.Time
}

type Member struct {
	UserID    uuid.UUID
	Email     string
	Name      string
	Role      string
	CreatedAt time.Time
}

type HouseholdWithRole struct {
	ID              uuid.UUID
	Name            string
	Role            string
	CreatedAt       time.Time
	MemberCreatedAt time.Time
}

type ListCursor struct {
	MemberCreatedAt time.Time
	HouseholdID     uuid.UUID
}
