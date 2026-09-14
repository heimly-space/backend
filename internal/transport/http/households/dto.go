package households

import (
	"time"
	"uuid"
)

type CreateRequest struct {
	Name string `json:"name"`
}

type InviteMemberRequest struct {
	Email string `json:"email"`
}

type HouseholdResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type HouseholdWithRoleResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type HouseholdsResponse struct {
	Items      []HouseholdWithRoleResponse `json:"items"`
	NextCursor string                      `json:"next_cursor,omitempty"`
}

type MemberResponse struct {
	UserID    uuid.UUID `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type MembersResponse struct {
	Members    []MemberResponse `json:"members"`
	NextCursor string           `json:"next_cursor,omitempty"`
}
