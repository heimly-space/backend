package households

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, name string, ownerID uuid.UUID) (*Household, error)
	Exists(ctx context.Context, householdID uuid.UUID) (bool, error)
	IsMember(ctx context.Context, householdID, userID uuid.UUID) (bool, error)
	AddMemberByEmail(ctx context.Context, householdID uuid.UUID, email string) (*Member, error)
	ListMembers(
		ctx context.Context,
		householdID uuid.UUID,
		cursor *MembersListCursor,
		limit int,
	) ([]Member, error)
	ListByUser(
		ctx context.Context,
		userID uuid.UUID,
		cursor *ListCursor,
		limit int,
	) ([]HouseholdWithRole, error)
}
