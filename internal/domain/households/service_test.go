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
}

func (r *repoStub) Create(ctx context.Context, name string, ownerID uuid.UUID) (*Household, error) {
	return r.createFn(ctx, name, ownerID)
}

func (r *repoStub) Exists(ctx context.Context, householdID uuid.UUID) (bool, error) {
	return r.existsFn(ctx, householdID)
}

func (r *repoStub) IsMember(ctx context.Context, householdID, userID uuid.UUID) (bool, error) {
	return r.isMemberFn(ctx, householdID, userID)
}

func (r *repoStub) AddMemberByEmail(ctx context.Context, householdID uuid.UUID, email string) (*Member, error) {
	return r.addMemberByEmailFn(ctx, householdID, email)
}

func (r *repoStub) ListMembers(ctx context.Context, householdID uuid.UUID) ([]Member, error) {
	return r.listMembersFn(ctx, householdID)
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
				return &Household{
					ID:        householdID,
					Name:      name,
					CreatedAt: time.Now().UTC(),
				}, nil
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
	svc := &Service{
		Repo: &repoStub{
			createFn: func(_ context.Context, _ string, _ uuid.UUID) (*Household, error) {
				t.Fatal("Create should not be called")
				return nil, nil
			},
		},
	}

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
			createFn: func(_ context.Context, _ string, _ uuid.UUID) (*Household, error) {
				t.Fatal("Create should not be called")
				return nil, nil
			},
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
				return &Member{
					UserID:    memberID,
					Email:     email,
					Name:      "White Rabbit",
					Role:      "member",
					CreatedAt: time.Now().UTC(),
				}, nil
			},
			listMembersFn: func(_ context.Context, _ uuid.UUID) ([]Member, error) {
				t.Fatal("ListMembers should not be called")
				return nil, nil
			},
		},
	}

	member, err := svc.InviteMember(
		context.Background(),
		householdID,
		actorID,
		"  rabbit@example.com ",
	)
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
			createFn: func(_ context.Context, _ string, _ uuid.UUID) (*Household, error) {
				t.Fatal("Create should not be called")
				return nil, nil
			},
			existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
				return true, nil
			},
			isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
				return false, nil
			},
			addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*Member, error) {
				t.Fatal("AddMemberByEmail should not be called")
				return nil, nil
			},
			listMembersFn: func(_ context.Context, _ uuid.UUID) ([]Member, error) {
				t.Fatal("ListMembers should not be called")
				return nil, nil
			},
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
			createFn: func(_ context.Context, _ string, _ uuid.UUID) (*Household, error) {
				t.Fatal("Create should not be called")
				return nil, nil
			},
			existsFn: func(_ context.Context, gotHouseholdID uuid.UUID) (bool, error) {
				return gotHouseholdID != householdID, nil
			},
			isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
				t.Fatal("IsMember should not be called when household is missing")
				return false, nil
			},
			addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*Member, error) {
				t.Fatal("AddMemberByEmail should not be called")
				return nil, nil
			},
			listMembersFn: func(_ context.Context, _ uuid.UUID) ([]Member, error) {
				t.Fatal("ListMembers should not be called")
				return nil, nil
			},
		},
	}

	_, err := svc.ListMembers(context.Background(), householdID, actorID)
	if !errors.Is(err, ErrHouseholdNotFound) {
		t.Fatalf("expected ErrHouseholdNotFound, got %v", err)
	}
}
