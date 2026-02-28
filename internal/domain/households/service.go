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

type ListMembersResult struct {
	Members    []Member
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
	cursorRaw string,
	limit int,
) (*ListMembersResult, error) {
	allowed, err := s.canAccessHousehold(ctx, householdID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	if err := s.ensureCursorSecretConfigured(); err != nil {
		return nil, err
	}

	listLimit := normalizeLimit(limit)
	cursor, err := parseMembersCursor(cursorRaw, s.CursorSecret)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}

	rows, err := s.Repo.ListMembers(ctx, householdID, cursor, listLimit+1)
	if err != nil {
		return nil, err
	}

	result := &ListMembersResult{Members: rows}
	if len(rows) > listLimit {
		last := rows[listLimit-1]
		result.Members = rows[:listLimit]
		result.NextCursor = encodeMembersCursor(MembersListCursor{
			MemberCreatedAt: last.CreatedAt,
			UserID:          last.UserID,
		}, s.CursorSecret)
	}

	return result, nil
}

func (s *Service) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	cursorRaw string,
	limit int,
) (*ListResult, error) {
	if err := s.ensureCursorSecretConfigured(); err != nil {
		return nil, err
	}

	listLimit := normalizeLimit(limit)
	cursor, err := parseHouseholdsCursor(cursorRaw, s.CursorSecret)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}

	rows, err := s.Repo.ListByUser(ctx, userID, cursor, listLimit+1)
	if err != nil {
		return nil, err
	}

	result := &ListResult{Items: rows}
	if len(rows) > listLimit {
		last := rows[listLimit-1]
		result.Items = rows[:listLimit]
		result.NextCursor = encodeHouseholdsCursor(ListCursor{
			MemberCreatedAt: last.MemberCreatedAt,
			HouseholdID:     last.ID,
		}, s.CursorSecret)
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

func (s *Service) ensureCursorSecretConfigured() error {
	if strings.TrimSpace(s.CursorSecret) == "" {
		return errors.New("cursor secret is not configured")
	}
	return nil
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

func parseHouseholdsCursor(raw, secret string) (*ListCursor, error) {
	t, id, err := parseTimeUUIDCursor(raw, secret)
	if err != nil {
		return nil, err
	}
	if id == uuid.Nil {
		return nil, nil
	}
	return &ListCursor{MemberCreatedAt: t, HouseholdID: id}, nil
}

func encodeHouseholdsCursor(cursor ListCursor, secret string) string {
	return encodeTimeUUIDCursor(cursor.MemberCreatedAt, cursor.HouseholdID, secret)
}

func parseMembersCursor(raw, secret string) (*MembersListCursor, error) {
	t, id, err := parseTimeUUIDCursor(raw, secret)
	if err != nil {
		return nil, err
	}
	if id == uuid.Nil {
		return nil, nil
	}
	return &MembersListCursor{MemberCreatedAt: t, UserID: id}, nil
}

func encodeMembersCursor(cursor MembersListCursor, secret string) string {
	return encodeTimeUUIDCursor(cursor.MemberCreatedAt, cursor.UserID, secret)
}

func parseTimeUUIDCursor(raw, secret string) (time.Time, uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, uuid.Nil, nil
	}

	first, second, err := parseSignedCursor(raw, secret)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}

	ns, err := strconv.ParseInt(first, 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	parsedID, err := uuid.Parse(second)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}

	return time.Unix(0, ns).UTC(), parsedID, nil
}

func encodeTimeUUIDCursor(t time.Time, id uuid.UUID, secret string) string {
	first := strconv.FormatInt(t.UTC().UnixNano(), 10)
	second := id.String()
	return encodeSignedCursor(first, second, secret)
}

func parseSignedCursor(raw, secret string) (string, string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", "", err
	}

	parts := strings.SplitN(string(decoded), "|", 3)
	if len(parts) != 3 {
		return "", "", fmt.Errorf("expected 3 parts")
	}

	payload := parts[0] + "|" + parts[1]
	if !validateCursorSignature(payload, parts[2], secret) {
		return "", "", fmt.Errorf("invalid signature")
	}

	return parts[0], parts[1], nil
}

func encodeSignedCursor(first, second, secret string) string {
	payload := first + "|" + second
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
