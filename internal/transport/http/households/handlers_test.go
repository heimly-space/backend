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
	listMembersFn      func(ctx context.Context, householdID uuid.UUID, cursor *domain.MembersListCursor, limit int) ([]domain.Member, error)
	listByUserFn       func(ctx context.Context, userID uuid.UUID, cursor *domain.ListCursor, limit int) ([]domain.HouseholdWithRole, error)
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

func (r *repoStub) AddMemberByEmail(ctx context.Context, householdID uuid.UUID, email string) (*domain.Member, error) {
	return r.addMemberByEmailFn(ctx, householdID, email)
}

func (r *repoStub) ListMembers(
	ctx context.Context,
	householdID uuid.UUID,
	cursor *domain.MembersListCursor,
	limit int,
) ([]domain.Member, error) {
	return r.listMembersFn(ctx, householdID, cursor, limit)
}

func (r *repoStub) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	cursor *domain.ListCursor,
	limit int,
) ([]domain.HouseholdWithRole, error) {
	return r.listByUserFn(ctx, userID, cursor, limit)
}

func TestCreateHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	createdAt := time.Date(2026, time.February, 24, 12, 34, 0, 0, time.UTC)

	h := &Handlers{Households: &domain.Service{Repo: &repoStub{
		createFn: func(_ context.Context, name string, ownerID uuid.UUID) (*domain.Household, error) {
			if ownerID != userID || name != "Mad Tea House" {
				t.Fatalf("unexpected create args: owner=%s name=%q", ownerID, name)
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
		listMembersFn: func(_ context.Context, _ uuid.UUID, _ *domain.MembersListCursor, _ int) ([]domain.Member, error) {
			t.Fatal("ListMembers should not be called")
			return nil, nil
		},
		listByUserFn: func(_ context.Context, _ uuid.UUID, _ *domain.ListCursor, _ int) ([]domain.HouseholdWithRole, error) {
			t.Fatal("ListByUser should not be called")
			return nil, nil
		},
	}}}

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

func TestCreateHandlerRejectsTrailingJSON(t *testing.T) {
	userID := uuid.New()

	h := &Handlers{Households: &domain.Service{Repo: &repoStub{
		createFn: func(_ context.Context, _ string, _ uuid.UUID) (*domain.Household, error) {
			t.Fatal("Create should not be called on invalid JSON")
			return nil, nil
		},
		existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) { return false, nil },
		isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
			return false, nil
		},
		addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) {
			return nil, nil
		},
		listMembersFn: func(_ context.Context, _ uuid.UUID, _ *domain.MembersListCursor, _ int) ([]domain.Member, error) {
			return nil, nil
		},
		listByUserFn: func(_ context.Context, _ uuid.UUID, _ *domain.ListCursor, _ int) ([]domain.HouseholdWithRole, error) {
			return nil, nil
		},
	}}}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/households",
		bytes.NewBufferString(`{"name":"Mad Tea House"} {"extra":true}`),
	)
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestInviteMemberHandlerConflict(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()

	h := &Handlers{Households: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
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
		addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) {
			return nil, domain.ErrMemberAlreadyExists
		},
		listMembersFn: func(_ context.Context, _ uuid.UUID, _ *domain.MembersListCursor, _ int) ([]domain.Member, error) {
			t.Fatal("ListMembers should not be called")
			return nil, nil
		},
		listByUserFn: func(_ context.Context, _ uuid.UUID, _ *domain.ListCursor, _ int) ([]domain.HouseholdWithRole, error) {
			t.Fatal("ListByUser should not be called")
			return nil, nil
		},
	}}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+householdID.String()+"/members", bytes.NewBufferString(`{"email":"alice@example.com"}`))
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.InviteMember(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestListMembersHandlerForbidden(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()

	h := &Handlers{Households: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		createFn: func(_ context.Context, _ string, _ uuid.UUID) (*domain.Household, error) { return nil, nil },
		existsFn: func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
		isMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) {
			return false, nil
		},
		addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) { return nil, nil },
		listMembersFn: func(_ context.Context, _ uuid.UUID, _ *domain.MembersListCursor, _ int) ([]domain.Member, error) {
			t.Fatal("ListMembers should not be called")
			return nil, nil
		},
		listByUserFn: func(_ context.Context, _ uuid.UUID, _ *domain.ListCursor, _ int) ([]domain.HouseholdWithRole, error) {
			return nil, nil
		},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+householdID.String()+"/members", nil)
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.ListMembers(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestListMembersHandlerPaginationSuccess(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	u1 := uuid.New()
	u2 := uuid.New()
	now := time.Date(2026, time.February, 24, 19, 0, 0, 0, time.UTC)

	var gotLimit int
	var gotCursor *domain.MembersListCursor

	h := &Handlers{Households: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		createFn:           func(_ context.Context, _ string, _ uuid.UUID) (*domain.Household, error) { return nil, nil },
		existsFn:           func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
		isMemberFn:         func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
		addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) { return nil, nil },
		listMembersFn: func(_ context.Context, _ uuid.UUID, cursor *domain.MembersListCursor, limit int) ([]domain.Member, error) {
			gotCursor = cursor
			gotLimit = limit
			return []domain.Member{{UserID: u1, Email: "a@x", Name: "A", Role: "owner", CreatedAt: now}, {UserID: u2, Email: "b@x", Name: "B", Role: "member", CreatedAt: now.Add(-time.Minute)}}, nil
		},
		listByUserFn: func(_ context.Context, _ uuid.UUID, _ *domain.ListCursor, _ int) ([]domain.HouseholdWithRole, error) {
			return nil, nil
		},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+householdID.String()+"/members?limit=1", nil)
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.ListMembers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if gotCursor != nil {
		t.Fatalf("expected nil cursor, got %+v", gotCursor)
	}
	if gotLimit != 2 {
		t.Fatalf("expected limit 2, got %d", gotLimit)
	}

	var resp MembersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(resp.Members))
	}
	if resp.NextCursor == "" {
		t.Fatal("expected next_cursor")
	}
}

func TestListMembersHandlerInvalidCursor(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()

	h := &Handlers{Households: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		createFn:           func(_ context.Context, _ string, _ uuid.UUID) (*domain.Household, error) { return nil, nil },
		existsFn:           func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
		isMemberFn:         func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
		addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) { return nil, nil },
		listMembersFn: func(_ context.Context, _ uuid.UUID, _ *domain.MembersListCursor, _ int) ([]domain.Member, error) {
			t.Fatal("ListMembers should not be called")
			return nil, nil
		},
		listByUserFn: func(_ context.Context, _ uuid.UUID, _ *domain.ListCursor, _ int) ([]domain.HouseholdWithRole, error) {
			return nil, nil
		},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+householdID.String()+"/members?cursor=bad", nil)
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.ListMembers(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestListByUserHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	h1 := uuid.New()
	h2 := uuid.New()
	createdAt := time.Date(2026, time.February, 24, 11, 0, 0, 0, time.UTC)

	var gotCursor *domain.ListCursor
	var gotLimit int

	h := &Handlers{Households: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		createFn:           func(_ context.Context, _ string, _ uuid.UUID) (*domain.Household, error) { return nil, nil },
		existsFn:           func(_ context.Context, _ uuid.UUID) (bool, error) { return false, nil },
		isMemberFn:         func(_ context.Context, _, _ uuid.UUID) (bool, error) { return false, nil },
		addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) { return nil, nil },
		listMembersFn: func(_ context.Context, _ uuid.UUID, _ *domain.MembersListCursor, _ int) ([]domain.Member, error) {
			return nil, nil
		},
		listByUserFn: func(_ context.Context, gotUserID uuid.UUID, cursor *domain.ListCursor, limit int) ([]domain.HouseholdWithRole, error) {
			if gotUserID != userID {
				t.Fatalf("unexpected user id: %s", gotUserID)
			}
			gotCursor = cursor
			gotLimit = limit
			return []domain.HouseholdWithRole{{ID: h1, Name: "Wonderland Flat", Role: "owner", CreatedAt: createdAt}, {ID: h2, Name: "Tea House", Role: "member", CreatedAt: createdAt.Add(-time.Hour)}}, nil
		},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/households?limit=10", nil)
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.ListByUser(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if gotCursor != nil {
		t.Fatalf("expected nil cursor, got %+v", gotCursor)
	}
	if gotLimit != 11 {
		t.Fatalf("expected limit 11, got %d", gotLimit)
	}
}

func TestListByUserHandlerInvalidLimit(t *testing.T) {
	h := &Handlers{Households: &domain.Service{Repo: &repoStub{}, CursorSecret: "cursor-secret"}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/households?limit=abc", nil)
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()
	h.ListByUser(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid limit") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestInviteMemberHandlerInvalidHouseholdID(t *testing.T) {
	h := &Handlers{Households: &domain.Service{Repo: &repoStub{}, CursorSecret: "cursor-secret"}}
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

	h := &Handlers{Households: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		createFn:           func(_ context.Context, _ string, _ uuid.UUID) (*domain.Household, error) { return nil, nil },
		existsFn:           func(_ context.Context, _ uuid.UUID) (bool, error) { return false, errors.New("db down") },
		isMemberFn:         func(_ context.Context, _, _ uuid.UUID) (bool, error) { return false, nil },
		addMemberByEmailFn: func(_ context.Context, _ uuid.UUID, _ string) (*domain.Member, error) { return nil, nil },
		listMembersFn: func(_ context.Context, _ uuid.UUID, _ *domain.MembersListCursor, _ int) ([]domain.Member, error) {
			return nil, nil
		},
		listByUserFn: func(_ context.Context, _ uuid.UUID, _ *domain.ListCursor, _ int) ([]domain.HouseholdWithRole, error) {
			return nil, nil
		},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+householdID.String()+"/members", nil)
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()
	h.ListMembers(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func withRouteID(r *http.Request, id string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, routeCtx))
}
