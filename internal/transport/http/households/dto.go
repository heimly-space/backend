package households

import "time"

type CreateRequest struct {
	Name string `json:"name"`
}

type InviteMemberRequest struct {
	Email string `json:"email"`
}

type HouseholdResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type MemberResponse struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type MembersResponse struct {
	Members []MemberResponse `json:"members"`
}
