package households

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	Repo         Repository
	CursorSecret string
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
	if strings.TrimSpace(s.CursorSecret) == "" {
		return nil, errors.New("cursor secret is not configured")
	}

	listLimit := normalizeLimit(limit)
	cursor, err := parseCursor(cursorRaw, s.CursorSecret)
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
		result.NextCursor = encodeCursor(
			ListCursor{
				MemberCreatedAt: last.MemberCreatedAt,
				HouseholdID:     last.ID,
			},
			s.CursorSecret,
		)
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

func parseCursor(raw, secret string) (*ListCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}

	parts := strings.SplitN(string(decoded), "|", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 parts")
	}

	ns, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return nil, err
	}
	householdID, err := uuid.Parse(parts[1])
	if err != nil {
		return nil, err
	}

	payload := parts[0] + "|" + parts[1]
	if !validateCursorSignature(payload, parts[2], secret) {
		return nil, fmt.Errorf("invalid signature")
	}

	return &ListCursor{
		MemberCreatedAt: time.Unix(0, ns).UTC(),
		HouseholdID:     householdID,
	}, nil
}

func encodeCursor(cursor ListCursor, secret string) string {
	payload := fmt.Sprintf("%d|%s", cursor.MemberCreatedAt.UTC().UnixNano(), cursor.HouseholdID.String())
	signature := signCursorPayload(payload, secret)
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + signature))
}

func signCursorPayload(payload, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func validateCursorSignature(payload, signature, secret string) bool {
	decodedSignature, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	expected := hmac.New(sha256.New, []byte(secret))
	expected.Write([]byte(payload))
	return hmac.Equal(decodedSignature, expected.Sum(nil))
}
