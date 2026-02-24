package households

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repoStub struct {
	createFn           func(ctx context.Context, name string, ownerID uuid.UUID) (*Household, error)
	existsFn           func(ctx context.Context, householdID uuid.UUID) (bool, error)
	isMemberFn         func(ctx context.Context, householdID, userID uuid.UUID) (bool, error)
	addMemberByEmailFn func(ctx context.Context, householdID uuid.UUID, email string) (*Member, error)
	listMembersFn      func(ctx context.Context, householdID uuid.UUID) ([]Member, error)
	listByUserFn       func(ctx context.Context, userID uuid.UUID, cursor *ListCursor, limit int) ([]HouseholdWithRole, error)
}

func (r *repoStub) Create(ctx context.Context, name string, ownerID uuid.UUID) (*Household, error) {
	if r.createFn == nil {
		return nil, errors.New("unexpected Create call")
	}
	return r.createFn(ctx, name, ownerID)
}

func (r *repoStub) Exists(ctx context.Context, householdID uuid.UUID) (bool, error) {
	if r.existsFn == nil {
		return false, errors.New("unexpected Exists call")
	}
	return r.existsFn(ctx, householdID)
}

func (r *repoStub) IsMember(ctx context.Context, householdID, userID uuid.UUID) (bool, error) {
	if r.isMemberFn == nil {
		return false, errors.New("unexpected IsMember call")
	}
	return r.isMemberFn(ctx, householdID, userID)
}

func (r *repoStub) AddMemberByEmail(ctx context.Context, householdID uuid.UUID, email string) (*Member, error) {
	if r.addMemberByEmailFn == nil {
		return nil, errors.New("unexpected AddMemberByEmail call")
	}
	return r.addMemberByEmailFn(ctx, householdID, email)
}

func (r *repoStub) ListMembers(ctx context.Context, householdID uuid.UUID) ([]Member, error) {
	if r.listMembersFn == nil {
		return nil, errors.New("unexpected ListMembers call")
	}
	return r.listMembersFn(ctx, householdID)
}

func (r *repoStub) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	cursor *ListCursor,
	limit int,
) ([]HouseholdWithRole, error) {
	if r.listByUserFn == nil {
		return nil, errors.New("unexpected ListByUser call")
	}
	return r.listByUserFn(ctx, userID, cursor, limit)
}

func TestServiceCreate(t *testing.T) {
	ownerID := uuid.New()
	householdID := uuid.New()
	var gotName string

	svc := &Service{
		Repo: &repoStub{
			createFn: func(_ context.Context, name string, gotOwnerID uuid.UUID) (*Household, error) {
				if gotOwnerID != ownerID {
					t.Fatalf("unexpected owner id: %s", gotOwnerID)
				}
				gotName = name
				return &Household{ID: householdID, Name: name, CreatedAt: time.Now().UTC()}, nil
			},
		},
	}

	h, err := svc.Create(context.Background(), ownerID, "  Wonderland Home ")
	if err != nil {
		t.Fatalf("create household: %v", err)
	}
	if h.ID != householdID {
		t.Fatalf("unexpected household id: %s", h.ID)
	}
	if gotName != "Wonderland Home" {
		t.Fatalf("expected trimmed name, got %q", gotName)
	}
}

func TestServiceCreateNameRequired(t *testing.T) {
	svc := &Service{Repo: &repoStub{}}

	_, err := svc.Create(context.Background(), uuid.New(), "   ")
	if !errors.Is(err, ErrHouseholdNameRequired) {
		t.Fatalf("expected ErrHouseholdNameRequired, got %v", err)
	}
}

func TestServiceInviteMember(t *testing.T) {
	householdID := uuid.New()
	actorID := uuid.New()
	memberID := uuid.New()
	var gotEmail string

	svc := &Service{
		Repo: &repoStub{
			existsFn: func(_ context.Context, gotHouseholdID uuid.UUID) (bool, error) {
				return gotHouseholdID == householdID, nil
			},
			isMemberFn: func(_ context.Context, gotHouseholdID, gotUserID uuid.UUID) (bool, error) {
				return gotHouseholdID == householdID && gotUserID == actorID, nil
			},
			addMemberByEmailFn: func(_ context.Context, gotHouseholdID uuid.UUID, email string) (*Member, error) {
				if gotHouseholdID != householdID {
					t.Fatalf("unexpected household id: %s", gotHouseholdID)
				}
				gotEmail = email
				return &Member{UserID: memberID, Email: email, Name: "White Rabbit", Role: "member", CreatedAt: time.Now().UTC()}, nil
			},
		},
	}

	member, err := svc.InviteMember(context.Background(), householdID, actorID, "  rabbit@example.com ")
	if err != nil {
		t.Fatalf("invite member: %v", err)
	}
	if member.UserID != memberID {
		t.Fatalf("unexpected member id: %s", member.UserID)
	}
	if gotEmail != "rabbit@example.com" {
		t.Fatalf("expected trimmed email, got %q", gotEmail)
	}
}

func TestServiceInviteMemberForbidden(t *testing.T) {
	householdID := uuid.New()
	actorID := uuid.New()

	svc := &Service{
		Repo: &repoStub{
			existsFn:   func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
			isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) { return false, nil },
		},
	}

	_, err := svc.InviteMember(context.Background(), householdID, actorID, "hatter@example.com")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceListMembersNotFound(t *testing.T) {
	householdID := uuid.New()
	actorID := uuid.New()

	svc := &Service{
		Repo: &repoStub{
			existsFn: func(_ context.Context, gotHouseholdID uuid.UUID) (bool, error) {
				return gotHouseholdID != householdID, nil
			},
			isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
				t.Fatal("IsMember should not be called when household is missing")
				return false, nil
			},
		},
	}

	_, err := svc.ListMembers(context.Background(), householdID, actorID)
	if !errors.Is(err, ErrHouseholdNotFound) {
		t.Fatalf("expected ErrHouseholdNotFound, got %v", err)
	}
}

func TestServiceListByUserDefaultLimitAndCursor(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, time.February, 24, 18, 0, 0, 0, time.UTC)
	id1 := uuid.New()
	id2 := uuid.New()

	var gotLimit int
	result, err := (&Service{
		Repo: &repoStub{
			listByUserFn: func(_ context.Context, gotUserID uuid.UUID, cursor *ListCursor, limit int) ([]HouseholdWithRole, error) {
				if gotUserID != userID {
					t.Fatalf("unexpected user id: %s", gotUserID)
				}
				if cursor != nil {
					t.Fatalf("expected nil cursor, got %+v", cursor)
				}
				gotLimit = limit
				return []HouseholdWithRole{
					{ID: id1, Name: "A", Role: "owner", MemberCreatedAt: now},
					{ID: id2, Name: "B", Role: "member", MemberCreatedAt: now.Add(-time.Minute)},
				}, nil
			},
		},
	}).ListByUser(context.Background(), userID, "", 0)
	if err != nil {
		t.Fatalf("list households: %v", err)
	}
	if gotLimit != defaultListLimit+1 {
		t.Fatalf("unexpected limit sent to repo: %d", gotLimit)
	}
	if len(result.Items) != 2 {
		t.Fatalf("unexpected items count: %d", len(result.Items))
	}
	if result.NextCursor != "" {
		t.Fatalf("unexpected next cursor: %q", result.NextCursor)
	}
}

func TestServiceListByUserPagination(t *testing.T) {
	userID := uuid.New()
	now := time.Date(2026, time.February, 24, 18, 0, 0, 0, time.UTC)
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	result, err := (&Service{
		Repo: &repoStub{
			listByUserFn: func(_ context.Context, _ uuid.UUID, _ *ListCursor, limit int) ([]HouseholdWithRole, error) {
				if limit != 3 {
					t.Fatalf("unexpected limit: %d", limit)
				}
				return []HouseholdWithRole{
					{ID: id1, Name: "A", Role: "owner", MemberCreatedAt: now},
					{ID: id2, Name: "B", Role: "member", MemberCreatedAt: now.Add(-time.Minute)},
					{ID: id3, Name: "C", Role: "member", MemberCreatedAt: now.Add(-2 * time.Minute)},
				}, nil
			},
		},
	}).ListByUser(context.Background(), userID, "", 2)
	if err != nil {
		t.Fatalf("list households: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.NextCursor == "" {
		t.Fatal("expected non-empty next cursor")
	}
}

func TestServiceListByUserInvalidCursor(t *testing.T) {
	svc := &Service{Repo: &repoStub{}}

	_, err := svc.ListByUser(context.Background(), uuid.New(), "bad-cursor", 10)
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}
