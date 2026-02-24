package households

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	Repo Repository
}

func (s *Service) Create(ctx context.Context, ownerID uuid.UUID, name string) (*Household, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrHouseholdNameRequired
	}
	return s.Repo.Create(ctx, name, ownerID)
}

func (s *Service) InviteMember(
	ctx context.Context,
	householdID, actorUserID uuid.UUID,
	email string,
) (*Member, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, ErrUserNotFound
	}

	allowed, err := s.canAccessHousehold(ctx, householdID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}

	return s.Repo.AddMemberByEmail(ctx, householdID, email)
}

func (s *Service) ListMembers(
	ctx context.Context,
	householdID, actorUserID uuid.UUID,
) ([]Member, error) {
	allowed, err := s.canAccessHousehold(ctx, householdID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	return s.Repo.ListMembers(ctx, householdID)
}

func (s *Service) canAccessHousehold(
	ctx context.Context,
	householdID, actorUserID uuid.UUID,
) (bool, error) {
	exists, err := s.Repo.Exists(ctx, householdID)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, ErrHouseholdNotFound
	}

	isMember, err := s.Repo.IsMember(ctx, householdID, actorUserID)
	if err != nil {
		return false, err
	}
	return isMember, nil
}
