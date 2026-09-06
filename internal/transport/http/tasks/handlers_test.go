package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	domain "heimly.space/backend/internal/domain/tasks"
	httpmw "heimly.space/backend/internal/transport/http/middleware"
)

type repoStub struct {
	householdExistsFn   func(ctx context.Context, householdID uuid.UUID) (bool, error)
	isHouseholdMemberFn func(ctx context.Context, householdID, userID uuid.UUID) (bool, error)
	createFn            func(ctx context.Context, task *domain.Task) (*domain.Task, error)
	listByHouseholdFn   func(ctx context.Context, householdID uuid.UUID, filter domain.ListFilter, cursor *domain.TaskListCursor, limit int) ([]domain.Task, error)
	getByIDFn           func(ctx context.Context, taskID uuid.UUID) (*domain.Task, error)
	updateFn            func(ctx context.Context, task *domain.Task, assigneesChanged bool) (*domain.Task, error)
	deleteFn            func(ctx context.Context, taskID uuid.UUID) (bool, error)
}

func (r *repoStub) HouseholdExists(ctx context.Context, householdID uuid.UUID) (bool, error) {
	if r.householdExistsFn == nil {
		return false, errors.New("unexpected HouseholdExists call")
	}
	return r.householdExistsFn(ctx, householdID)
}

func (r *repoStub) IsHouseholdMember(ctx context.Context, householdID, userID uuid.UUID) (bool, error) {
	if r.isHouseholdMemberFn == nil {
		return false, errors.New("unexpected IsHouseholdMember call")
	}
	return r.isHouseholdMemberFn(ctx, householdID, userID)
}

func (r *repoStub) Create(ctx context.Context, task *domain.Task) (*domain.Task, error) {
	if r.createFn == nil {
		return nil, errors.New("unexpected Create call")
	}
	return r.createFn(ctx, task)
}

func (r *repoStub) ListByHousehold(
	ctx context.Context,
	householdID uuid.UUID,
	filter domain.ListFilter,
	cursor *domain.TaskListCursor,
	limit int,
) ([]domain.Task, error) {
	if r.listByHouseholdFn == nil {
		return nil, errors.New("unexpected ListByHousehold call")
	}
	return r.listByHouseholdFn(ctx, householdID, filter, cursor, limit)
}

func (r *repoStub) GetByID(ctx context.Context, taskID uuid.UUID) (*domain.Task, error) {
	if r.getByIDFn == nil {
		return nil, errors.New("unexpected GetByID call")
	}
	return r.getByIDFn(ctx, taskID)
}

func (r *repoStub) Update(ctx context.Context, task *domain.Task, assigneesChanged bool) (*domain.Task, error) {
	if r.updateFn == nil {
		return nil, errors.New("unexpected Update call")
	}
	return r.updateFn(ctx, task, assigneesChanged)
}

func (r *repoStub) Delete(ctx context.Context, taskID uuid.UUID) (bool, error) {
	if r.deleteFn == nil {
		return false, errors.New("unexpected Delete call")
	}
	return r.deleteFn(ctx, taskID)
}

func TestCreateTaskHandlerSuccess(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	a1 := uuid.New()
	a2 := uuid.New()
	taskID := uuid.New()
	now := time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC)

	h := &Handlers{Tasks: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		householdExistsFn:   func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
		isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
		createFn: func(_ context.Context, task *domain.Task) (*domain.Task, error) {
			if task.Title != "Serve tea at the Mad Tea Party" {
				t.Fatalf("unexpected title: %q", task.Title)
			}
			if len(task.AssigneeIDs) != 2 {
				t.Fatalf("expected 2 assignees, got %d", len(task.AssigneeIDs))
			}
			task.ID = taskID
			task.CreatedAt = now
			task.UpdatedAt = now
			return task, nil
		},
	}}}

	body := `{"title":"Serve tea at the Mad Tea Party","assignee_ids":["` + a1.String() + `","` + a2.String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/households/"+householdID.String()+"/tasks", bytes.NewBufferString(body))
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", rec.Code, rec.Body.String())
	}

	var resp TaskResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ID != taskID.String() {
		t.Fatalf("unexpected id: %s", resp.ID)
	}
	if len(resp.AssigneeIDs) != 2 {
		t.Fatalf("expected 2 assignee_ids, got %d", len(resp.AssigneeIDs))
	}
}

func TestListTasksHandlerPaginationSuccess(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()
	t1 := uuid.New()
	t2 := uuid.New()
	t3 := uuid.New()
	now := time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC)

	var gotCursor *domain.TaskListCursor
	var gotLimit int

	h := &Handlers{Tasks: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		householdExistsFn:   func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
		isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
		listByHouseholdFn: func(_ context.Context, _ uuid.UUID, filter domain.ListFilter, cursor *domain.TaskListCursor, limit int) ([]domain.Task, error) {
			if filter.Status != domain.StatusPending {
				t.Fatalf("unexpected filter status: %q", filter.Status)
			}
			gotCursor = cursor
			gotLimit = limit
			return []domain.Task{
				{ID: t1, HouseholdID: householdID, Title: "Find the White Rabbit", Status: domain.StatusPending, CreatedAt: now, UpdatedAt: now},
				{ID: t2, HouseholdID: householdID, Title: "Paint the white roses", Status: domain.StatusPending, CreatedAt: now.Add(-time.Minute), UpdatedAt: now},
				{ID: t3, HouseholdID: householdID, Title: "Prepare tea for the Hatter", Status: domain.StatusPending, CreatedAt: now.Add(-2 * time.Minute), UpdatedAt: now},
			}, nil
		},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+householdID.String()+"/tasks?status=pending&limit=2", nil)
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	h.ListByHousehold(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	if gotCursor != nil {
		t.Fatalf("expected nil cursor, got %+v", gotCursor)
	}
	if gotLimit != 3 {
		t.Fatalf("expected limit 3, got %d", gotLimit)
	}

	var resp ListTasksResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.NextCursor == "" {
		t.Fatal("expected next_cursor")
	}
}

func TestListTasksHandlerInvalidCursor(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()

	h := &Handlers{Tasks: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		householdExistsFn:   func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
		isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
		listByHouseholdFn: func(_ context.Context, _ uuid.UUID, _ domain.ListFilter, _ *domain.TaskListCursor, _ int) ([]domain.Task, error) {
			t.Fatal("repo should not be called")
			return nil, nil
		},
	}}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+householdID.String()+"/tasks?cursor=bad", nil)
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	h.ListByHousehold(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestUpdateTaskHandlerClearAssignees(t *testing.T) {
	userID := uuid.New()
	taskID := uuid.New()
	householdID := uuid.New()

	h := &Handlers{Tasks: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Task, error) {
			return &domain.Task{ID: taskID, HouseholdID: householdID, Title: "Old Rabbit Reminder", Status: domain.StatusPending, AssigneeIDs: []uuid.UUID{uuid.New()}}, nil
		},
		isHouseholdMemberFn: func(_ context.Context, _, gotUserID uuid.UUID) (bool, error) {
			return gotUserID == userID, nil
		},
		updateFn: func(_ context.Context, task *domain.Task, assigneesChanged bool) (*domain.Task, error) {
			if !assigneesChanged {
				t.Fatal("expected assignees changed")
			}
			if len(task.AssigneeIDs) != 0 {
				t.Fatalf("expected no assignees, got %d", len(task.AssigneeIDs))
			}
			if task.Status != domain.StatusDone {
				t.Fatalf("unexpected status: %s", task.Status)
			}
			return task, nil
		},
	}}}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tasks/"+taskID.String(), bytes.NewBufferString(`{"status":"done","assignee_ids":null}`))
	req = withRouteID(req, taskID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestListTasksHandlerInvalidAssignee(t *testing.T) {
	h := &Handlers{Tasks: &domain.Service{Repo: &repoStub{}}}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/households/"+uuid.NewString()+"/tasks?assignee=bad", nil)
	req = withRouteID(req, uuid.NewString())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), uuid.New()))
	rec := httptest.NewRecorder()

	h.ListByHousehold(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateTaskHandlerRejectsTrailingJSON(t *testing.T) {
	userID := uuid.New()
	householdID := uuid.New()

	h := &Handlers{Tasks: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		householdExistsFn:   func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
		isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
		createFn: func(_ context.Context, _ *domain.Task) (*domain.Task, error) {
			t.Fatal("Create should not be called for invalid JSON")
			return nil, nil
		},
	}}}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/households/"+householdID.String()+"/tasks",
		bytes.NewBufferString(`{"title":"Paint the white roses"} {"extra":true}`),
	)
	req = withRouteID(req, householdID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestUpdateTaskHandlerInvalidAssigneeIDs(t *testing.T) {
	userID := uuid.New()
	taskID := uuid.New()
	h := &Handlers{Tasks: &domain.Service{Repo: &repoStub{}}}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tasks/"+taskID.String(), bytes.NewBufferString(`{"assignee_ids":["bad"]}`))
	req = withRouteID(req, taskID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestDeleteTaskHandlerNotFound(t *testing.T) {
	userID := uuid.New()
	taskID := uuid.New()

	h := &Handlers{Tasks: &domain.Service{CursorSecret: "cursor-secret", Repo: &repoStub{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Task, error) { return nil, nil },
	}}}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/tasks/"+taskID.String(), nil)
	req = withRouteID(req, taskID.String())
	req = req.WithContext(httpmw.ContextWithUserID(req.Context(), userID))
	rec := httptest.NewRecorder()

	h.Delete(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func withRouteID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
