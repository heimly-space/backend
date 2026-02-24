package households

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	Repo Repository
}

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

type ListResult struct {
	Items      []HouseholdWithRole
	NextCursor string
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

func (s *Service) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	cursorRaw string,
	limit int,
) (*ListResult, error) {
	listLimit := normalizeLimit(limit)
	cursor, err := parseCursor(cursorRaw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}

	rows, err := s.Repo.ListByUser(ctx, userID, cursor, listLimit+1)
	if err != nil {
		return nil, err
	}

	result := &ListResult{
		Items:      rows,
		NextCursor: "",
	}
	if len(rows) > listLimit {
		last := rows[listLimit-1]
		result.Items = rows[:listLimit]
		result.NextCursor = encodeCursor(ListCursor{
			MemberCreatedAt: last.MemberCreatedAt,
			HouseholdID:     last.ID,
		})
	}

	return result, nil
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

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

func parseCursor(raw string) (*ListCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected 2 parts")
	}

	ns, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}
	householdID, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, err
	}

	return &ListCursor{
		MemberCreatedAt: time.Unix(0, ns).UTC(),
		HouseholdID:     householdID,
	}, nil
}

func encodeCursor(cursor ListCursor) string {
	payload := fmt.Sprintf("%d|%s", cursor.MemberCreatedAt.UTC().UnixNano(), cursor.HouseholdID.String())
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}
