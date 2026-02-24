package households

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	domain "heimly.space/backend/internal/domain/households"
	httpmw "heimly.space/backend/internal/transport/http/middleware"
)

type repoStub struct {
	createFn           func(ctx context.Context, name string, ownerID uuid.UUID) (*domain.Household, error)
	existsFn           func(ctx context.Context, householdID uuid.UUID) (bool, error)
	isMemberFn         func(ctx context.Context, householdID, userID uuid.UUID) (bool, error)
	addMemberByEmailFn func(ctx context.Context, householdID uuid.UUID, email string) (*domain.Member, error)
	listMembersFn      func(ctx context.Context, householdID uuid.UUID) ([]domain.Member, error)
}

func (r *repoStub) Create(ctx context.Context, name string, ownerID uuid.UUID) (*domain.Household, error) {
	return r.createFn(ctx, name, ownerID)
}

func (r *repoStub) Exists(ctx context.Context, householdID uuid.UUID) (bool, error) {
	return r.existsFn(ctx, householdID)
}

func (r *repoStub) IsMember(ctx context.Context, householdID, userID uuid.UUID) (bool, error) {
	return r.isMemberFn(ctx, householdID, userID)
}

func (r *repoStub) AddMemberByEmail(
	ctx context.Context,
	householdID uuid.UUID,
	email string,
) (*domain.Member, error) {
	return r.addMemberByEmailFn(ctx, householdID, email)
}

func (r *repoStub) ListMembers(ctx context.Context, householdID uuid.UUID) ([]domain.Member, error) {
	return r.listMembersFn(ctx, householdID)
}

func TestCreateHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	createdAt := time.Date(2026, time.February, 24, 12, 34, 0, 0, time.UTC)

	h := &Handlers{
		Households: &domain.Service{
			Repo: &repoStub{
				createFn: func(_ context.Context, name string, ownerID uuid.UUID) (*domain.Household, error) {
					if ownerID != userID {
						t.Fatalf("unexpected owner id: %s", ownerID)
					}
					if name != "Mad Tea House" {
						t.Fatalf("unexpected household name: %q", name)
					}
					return &domain.Household{ID: householdID, Name: name, CreatedAt: createdAt}, nil
				},
				existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
					t.Fatal("Exists should not be called")
					return false, nil
				},
				isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
					t.Fatal("IsMember should not be called")
					return false, nil
				},
				addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) {
					t.Fatal("AddMemberByEmail should not be called")
					return nil, nil
				},
				listMembersFn: func(_ context.Context, _ uuid.UUID) ([]domain.Member, error) {
					t.Fatal("ListMembers should not be called")
					return nil, nil
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/households", bytes.NewBufferString(`{"name":"Mad Tea House"}`))
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp HouseholdResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != householdID.String() || resp.Name != "Mad Tea House" || !resp.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestInviteMemberHandlerConflict(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()

	h := &Handlers{
		Households: &domain.Service{
			Repo: &repoStub{
				createFn: func(_ context.Context, _ string, _ uuid.UUID) (*domain.Household, error) {
					t.Fatal("Create should not be called")
					return nil, nil
				},
				existsFn: func(_ context.Context, gotHouseholdID uuid.UUID) (bool, error) {
					return gotHouseholdID == householdID, nil
				},
				isMemberFn: func(_ context.Context, gotHouseholdID, gotUserID uuid.UUID) (bool, error) {
					return gotHouseholdID == householdID && gotUserID == userID, nil
				},
				addMemberByEmailFn: func(_ context.Context, gotHouseholdID uuid.UUID, email string) (*domain.Member, error) {
					if gotHouseholdID != householdID || email != "alice@example.com" {
						t.Fatalf("unexpected invite params: %s %q", gotHouseholdID, email)
					}
					return nil, domain.ErrMemberAlreadyExists
				},
				listMembersFn: func(_ context.Context, _ uuid.UUID) ([]domain.Member, error) {
					t.Fatal("ListMembers should not be called")
					return nil, nil
				},
			},
		},
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/households/"+householdID.String()+"/members",
		bytes.NewBufferString(`{"email":"alice@example.com"}`),
	)
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.InviteMember(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "member already exists") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestListMembersHandlerForbidden(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()

	h := &Handlers{
		Households: &domain.Service{
			Repo: &repoStub{
				createFn: func(_ context.Context, _ string, _ uuid.UUID) (*domain.Household, error) {
					t.Fatal("Create should not be called")
					return nil, nil
				},
				existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
					return true, nil
				},
				isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
					return false, nil
				},
				addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) {
					t.Fatal("AddMemberByEmail should not be called")
					return nil, nil
				},
				listMembersFn: func(_ context.Context, _ uuid.UUID) ([]domain.Member, error) {
					t.Fatal("ListMembers should not be called")
					return nil, nil
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+householdID.String()+"/members", nil)
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.ListMembers(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func withRouteID(r *http.Request, id string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx))
}

func TestInviteMemberHandlerInvalidHouseholdID(t *testing.T) {
	h := &Handlers{
		Households: &domain.Service{
			Repo: &repoStub{
				createFn: func(_ context.Context, _ string, _ uuid.UUID) (*domain.Household, error) {
					t.Fatal("Create should not be called")
					return nil, nil
				},
				existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
					t.Fatal("Exists should not be called")
					return false, nil
				},
				isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
					t.Fatal("IsMember should not be called")
					return false, nil
				},
				addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) {
					t.Fatal("AddMemberByEmail should not be called")
					return nil, nil
				},
				listMembersFn: func(_ context.Context, _ uuid.UUID) ([]domain.Member, error) {
					t.Fatal("ListMembers should not be called")
					return nil, nil
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/households/not-uuid/members", bytes.NewBufferString(`{"email":"a@b.c"}`))
	req = withRouteID(req, "not-uuid")
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()
	h.InviteMember(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListMembersHandlerInternalError(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()

	h := &Handlers{
		Households: &domain.Service{
			Repo: &repoStub{
				createFn: func(_ context.Context, _ string, _ uuid.UUID) (*domain.Household, error) {
					t.Fatal("Create should not be called")
					return nil, nil
				},
				existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
					return false, errors.New("db down")
				},
				isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
					t.Fatal("IsMember should not be called")
					return false, nil
				},
				addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) {
					t.Fatal("AddMemberByEmail should not be called")
					return nil, nil
				},
				listMembersFn: func(_ context.Context, _ uuid.UUID) ([]domain.Member, error) {
					t.Fatal("ListMembers should not be called")
					return nil, nil
				},
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+householdID.String()+"/members", nil)
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.ListMembers(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
