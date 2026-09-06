package tasks

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

func (s *Service) Create(
	ctx context.Context,
	householdID, actorUserID uuid.UUID,
	input CreateTaskInput,
) (*Task, error) {
	status, err := normalizeStatus(input.Status, true)
	if err != nil {
		return nil, err
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, ErrTaskTitleRequired
	}

	allowed, err := s.canAccessHousehold(ctx, householdID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}

	assigneeIDs := normalizeAssigneeIDs(input.AssigneeIDs)
	if err := s.validateAssignees(ctx, householdID, assigneeIDs); err != nil {
		return nil, err
	}

	task := &Task{
		HouseholdID: householdID,
		Title:       title,
		Description: input.Description,
		Status:      status,
		DueAt:       normalizeDate(input.DueAt),
		AssigneeIDs: assigneeIDs,
	}

	return s.Repo.Create(ctx, task)
}

func (s *Service) ListByHousehold(
	ctx context.Context,
	householdID, actorUserID uuid.UUID,
	filter ListFilter,
	cursorRaw string,
	limit int,
) (*ListResult, error) {
	status, err := normalizeStatus(filter.Status, false)
	if err != nil {
		return nil, err
	}

	allowed, err := s.canAccessHousehold(ctx, householdID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}

	if filter.AssigneeID != nil {
		isMember, err := s.Repo.IsHouseholdMember(ctx, householdID, *filter.AssigneeID)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, ErrAssigneeNotInHousehold
		}
	}

	if err := s.ensureCursorSecretConfigured(); err != nil {
		return nil, err
	}

	listLimit := normalizeLimit(limit)
	cursor, err := parseCursor(cursorRaw, s.CursorSecret)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCursor, err)
	}

	rows, err := s.Repo.ListByHousehold(
		ctx,
		householdID,
		ListFilter{Status: status, AssigneeID: filter.AssigneeID},
		cursor,
		listLimit+1,
	)
	if err != nil {
		return nil, err
	}

	result := &ListResult{Items: rows}
	if len(rows) > listLimit {
		last := rows[listLimit-1]
		result.Items = rows[:listLimit]
		result.NextCursor = encodeCursor(TaskListCursor{
			CreatedAt: last.CreatedAt,
			TaskID:    last.ID,
		}, s.CursorSecret)
	}

	return result, nil
}

func (s *Service) Update(
	ctx context.Context,
	taskID, actorUserID uuid.UUID,
	patch PatchTaskInput,
) (*Task, error) {
	if patch.Title == nil && patch.Description == nil && patch.Status == nil && !patch.DueAtSet && !patch.AssigneesSet {
		return nil, ErrTaskPatchEmpty
	}

	task, err := s.Repo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrTaskNotFound
	}

	isActorMember, err := s.Repo.IsHouseholdMember(ctx, task.HouseholdID, actorUserID)
	if err != nil {
		return nil, err
	}
	if !isActorMember {
		return nil, ErrForbidden
	}

	updated := *task
	if patch.Title != nil {
		title := strings.TrimSpace(*patch.Title)
		if title == "" {
			return nil, ErrTaskTitleRequired
		}
		updated.Title = title
	}
	if patch.Description != nil {
		updated.Description = *patch.Description
	}
	if patch.Status != nil {
		status, err := normalizeStatus(*patch.Status, false)
		if err != nil {
			return nil, err
		}
		updated.Status = status
	}
	if patch.DueAtSet {
		if patch.DueAt == nil {
			updated.DueAt = time.Time{}
		} else {
			updated.DueAt = normalizeDate(*patch.DueAt)
		}
	}
	if patch.AssigneesSet {
		assigneeIDs := normalizeAssigneeIDs(patch.AssigneeIDs)
		if err := s.validateAssignees(ctx, task.HouseholdID, assigneeIDs); err != nil {
			return nil, err
		}
		updated.AssigneeIDs = assigneeIDs
	}

	saved, err := s.Repo.Update(ctx, &updated, patch.AssigneesSet)
	if err != nil {
		return nil, err
	}
	if saved == nil {
		return nil, ErrTaskNotFound
	}

	return saved, nil
}

func (s *Service) Delete(ctx context.Context, taskID, actorUserID uuid.UUID) error {
	task, err := s.Repo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return ErrTaskNotFound
	}

	isActorMember, err := s.Repo.IsHouseholdMember(ctx, task.HouseholdID, actorUserID)
	if err != nil {
		return err
	}
	if !isActorMember {
		return ErrForbidden
	}

	deleted, err := s.Repo.Delete(ctx, taskID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrTaskNotFound
	}

	return nil
}

func (s *Service) canAccessHousehold(
	ctx context.Context,
	householdID, actorUserID uuid.UUID,
) (bool, error) {
	exists, err := s.Repo.HouseholdExists(ctx, householdID)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, ErrHouseholdNotFound
	}

	isMember, err := s.Repo.IsHouseholdMember(ctx, householdID, actorUserID)
	if err != nil {
		return false, err
	}

	return isMember, nil
}

func (s *Service) validateAssignees(ctx context.Context, householdID uuid.UUID, assigneeIDs []uuid.UUID) error {
	for _, assigneeID := range assigneeIDs {
		isMember, err := s.Repo.IsHouseholdMember(ctx, householdID, assigneeID)
		if err != nil {
			return err
		}
		if !isMember {
			return ErrAssigneeNotInHousehold
		}
	}
	return nil
}

func (s *Service) ensureCursorSecretConfigured() error {
	if strings.TrimSpace(s.CursorSecret) == "" {
		return errors.New("cursor secret is not configured")
	}
	return nil
}

func normalizeStatus(raw string, allowEmpty bool) (string, error) {
	status := strings.ToLower(strings.TrimSpace(raw))
	if status == "" {
		if allowEmpty {
			return StatusPending, nil
		}
		return "", nil
	}

	switch status {
	case StatusPending, StatusInProgress, StatusDone:
		return status, nil
	default:
		return "", ErrTaskStatusInvalid
	}
}

func normalizeDate(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return t.UTC()
}

func normalizeAssigneeIDs(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return nil
	}

	seen := make(map[uuid.UUID]struct{}, len(ids))
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	if len(result) == 0 {
		return nil
	}
	return result
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

func parseCursor(raw, secret string) (*TaskListCursor, error) {
	t, id, err := parseTimeUUIDCursor(raw, secret)
	if err != nil {
		return nil, err
	}
	if id == uuid.Nil {
		return nil, nil
	}
	return &TaskListCursor{CreatedAt: t, TaskID: id}, nil
}

func encodeCursor(cursor TaskListCursor, secret string) string {
	return encodeTimeUUIDCursor(cursor.CreatedAt, cursor.TaskID, secret)
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
