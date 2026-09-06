package tasks

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repoStub struct {
	householdExistsFn   func(ctx context.Context, householdID uuid.UUID) (bool, error)
	isHouseholdMemberFn func(ctx context.Context, householdID, userID uuid.UUID) (bool, error)
	createFn            func(ctx context.Context, task *Task) (*Task, error)
	listByHouseholdFn   func(ctx context.Context, householdID uuid.UUID, filter ListFilter, cursor *TaskListCursor, limit int) ([]Task, error)
	getByIDFn           func(ctx context.Context, taskID uuid.UUID) (*Task, error)
	updateFn            func(ctx context.Context, task *Task, assigneesChanged bool) (*Task, error)
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

func (r *repoStub) Create(ctx context.Context, task *Task) (*Task, error) {
	if r.createFn == nil {
		return nil, errors.New("unexpected Create call")
	}
	return r.createFn(ctx, task)
}

func (r *repoStub) ListByHousehold(
	ctx context.Context,
	householdID uuid.UUID,
	filter ListFilter,
	cursor *TaskListCursor,
	limit int,
) ([]Task, error) {
	if r.listByHouseholdFn == nil {
		return nil, errors.New("unexpected ListByHousehold call")
	}
	return r.listByHouseholdFn(ctx, householdID, filter, cursor, limit)
}

func (r *repoStub) GetByID(ctx context.Context, taskID uuid.UUID) (*Task, error) {
	if r.getByIDFn == nil {
		return nil, errors.New("unexpected GetByID call")
	}
	return r.getByIDFn(ctx, taskID)
}

func (r *repoStub) Update(ctx context.Context, task *Task, assigneesChanged bool) (*Task, error) {
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

func TestCreateTaskWithAssigneesDeduplicates(t *testing.T) {
	householdID := uuid.New()
	actorID := uuid.New()
	a1 := uuid.New()
	a2 := uuid.New()

	checked := map[uuid.UUID]int{}
	svc := &Service{
		Repo: &repoStub{
			householdExistsFn: func(_ context.Context, gotHouseholdID uuid.UUID) (bool, error) {
				return gotHouseholdID == householdID, nil
			},
			isHouseholdMemberFn: func(_ context.Context, gotHouseholdID, gotUserID uuid.UUID) (bool, error) {
				if gotHouseholdID != householdID {
					t.Fatalf("unexpected household id: %s", gotHouseholdID)
				}
				checked[gotUserID]++
				return true, nil
			},
			createFn: func(_ context.Context, task *Task) (*Task, error) {
				if len(task.AssigneeIDs) != 2 {
					t.Fatalf("expected 2 assignees, got %d", len(task.AssigneeIDs))
				}
				if task.AssigneeIDs[0] != a1 || task.AssigneeIDs[1] != a2 {
					t.Fatalf("unexpected assignees order: %+v", task.AssigneeIDs)
				}
				return task, nil
			},
		},
	}

	_, err := svc.Create(context.Background(), householdID, actorID, CreateTaskInput{
		Title:       "Paint the white roses",
		AssigneeIDs: []uuid.UUID{a1, a1, uuid.Nil, a2},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if checked[a1] != 1 || checked[a2] != 1 {
		t.Fatalf("expected one membership check per assignee, got: %+v", checked)
	}
}

func TestListByHouseholdPagination(t *testing.T) {
	householdID := uuid.New()
	actorID := uuid.New()
	t1 := uuid.New()
	t2 := uuid.New()
	t3 := uuid.New()
	now := time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC)

	result, err := (&Service{
		Repo: &repoStub{
			householdExistsFn:   func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
			isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
			listByHouseholdFn: func(_ context.Context, _ uuid.UUID, filter ListFilter, cursor *TaskListCursor, limit int) ([]Task, error) {
				if filter.Status != StatusPending {
					t.Fatalf("unexpected filter status: %q", filter.Status)
				}
				if cursor != nil {
					t.Fatalf("expected nil cursor, got %+v", cursor)
				}
				if limit != 3 {
					t.Fatalf("unexpected limit: %d", limit)
				}
				return []Task{
					{ID: t1, CreatedAt: now},
					{ID: t2, CreatedAt: now.Add(-time.Minute)},
					{ID: t3, CreatedAt: now.Add(-2 * time.Minute)},
				}, nil
			},
		},
		CursorSecret: "cursor-secret",
	}).ListByHousehold(context.Background(), householdID, actorID, ListFilter{Status: "pending"}, "", 2)
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.NextCursor == "" {
		t.Fatal("expected next cursor")
	}
}

func TestListByHouseholdWithCursor(t *testing.T) {
	householdID := uuid.New()
	actorID := uuid.New()
	cursorTime := time.Date(2026, time.February, 28, 11, 0, 0, 0, time.UTC)
	cursorTaskID := uuid.New()
	cursorRaw := encodeCursor(TaskListCursor{CreatedAt: cursorTime, TaskID: cursorTaskID}, "cursor-secret")

	_, err := (&Service{
		Repo: &repoStub{
			householdExistsFn:   func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
			isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
			listByHouseholdFn: func(_ context.Context, _ uuid.UUID, _ ListFilter, cursor *TaskListCursor, limit int) ([]Task, error) {
				if cursor == nil {
					t.Fatal("expected cursor")
				}
				if !cursor.CreatedAt.Equal(cursorTime) || cursor.TaskID != cursorTaskID {
					t.Fatalf("unexpected cursor: %+v", cursor)
				}
				if limit != defaultListLimit+1 {
					t.Fatalf("unexpected limit: %d", limit)
				}
				return nil, nil
			},
		},
		CursorSecret: "cursor-secret",
	}).ListByHousehold(context.Background(), householdID, actorID, ListFilter{}, cursorRaw, 0)
	if err != nil {
		t.Fatalf("list tasks with cursor: %v", err)
	}
}

func TestListByHouseholdInvalidCursor(t *testing.T) {
	svc := &Service{
		Repo: &repoStub{
			householdExistsFn:   func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
			isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
			listByHouseholdFn: func(_ context.Context, _ uuid.UUID, _ ListFilter, _ *TaskListCursor, _ int) ([]Task, error) {
				t.Fatal("repo should not be called")
				return nil, nil
			},
		},
		CursorSecret: "cursor-secret",
	}

	_, err := svc.ListByHousehold(context.Background(), uuid.New(), uuid.New(), ListFilter{}, "bad", 10)
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestListByHouseholdRejectsTamperedCursor(t *testing.T) {
	householdID := uuid.New()
	actorID := uuid.New()
	cursorRaw := encodeCursor(TaskListCursor{
		CreatedAt: time.Date(2026, time.February, 28, 11, 0, 0, 0, time.UTC),
		TaskID:    uuid.New(),
	}, "cursor-secret")

	svc := &Service{
		Repo: &repoStub{
			householdExistsFn:   func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
			isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
			listByHouseholdFn: func(_ context.Context, _ uuid.UUID, _ ListFilter, _ *TaskListCursor, _ int) ([]Task, error) {
				t.Fatal("repo should not be called")
				return nil, nil
			},
		},
		CursorSecret: "cursor-secret",
	}

	_, err := svc.ListByHousehold(context.Background(), householdID, actorID, ListFilter{}, cursorRaw+"tamper", 10)
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("expected ErrInvalidCursor, got %v", err)
	}
}

func TestListByHouseholdCursorSecretRequired(t *testing.T) {
	svc := &Service{
		Repo: &repoStub{
			householdExistsFn:   func(_ context.Context, _ uuid.UUID) (bool, error) { return true, nil },
			isHouseholdMemberFn: func(_ context.Context, _, _ uuid.UUID) (bool, error) { return true, nil },
			listByHouseholdFn: func(_ context.Context, _ uuid.UUID, _ ListFilter, _ *TaskListCursor, _ int) ([]Task, error) {
				t.Fatal("repo should not be called")
				return nil, nil
			},
		},
	}

	_, err := svc.ListByHousehold(context.Background(), uuid.New(), uuid.New(), ListFilter{}, "", 10)
	if err == nil || err.Error() != "cursor secret is not configured" {
		t.Fatalf("expected cursor secret error, got %v", err)
	}
}

func TestUpdateTaskAssigneeNotInHousehold(t *testing.T) {
	taskID := uuid.New()
	householdID := uuid.New()
	actorID := uuid.New()
	a1 := uuid.New()

	svc := &Service{Repo: &repoStub{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*Task, error) {
			return &Task{ID: taskID, HouseholdID: householdID, Title: "Old Rabbit Reminder", Status: StatusPending}, nil
		},
		isHouseholdMemberFn: func(_ context.Context, _, gotUserID uuid.UUID) (bool, error) {
			if gotUserID == actorID {
				return true, nil
			}
			return false, nil
		},
	}}

	_, err := svc.Update(context.Background(), taskID, actorID, PatchTaskInput{AssigneesSet: true, AssigneeIDs: []uuid.UUID{a1}})
	if !errors.Is(err, ErrAssigneeNotInHousehold) {
		t.Fatalf("expected ErrAssigneeNotInHousehold, got %v", err)
	}
}

func TestUpdateTaskClearAssignees(t *testing.T) {
	taskID := uuid.New()
	householdID := uuid.New()
	actorID := uuid.New()
	status := "done"

	svc := &Service{Repo: &repoStub{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*Task, error) {
			return &Task{ID: taskID, HouseholdID: householdID, Title: "Old Rabbit Reminder", Status: StatusPending, AssigneeIDs: []uuid.UUID{uuid.New()}}, nil
		},
		isHouseholdMemberFn: func(_ context.Context, _, gotUserID uuid.UUID) (bool, error) {
			return gotUserID == actorID, nil
		},
		updateFn: func(_ context.Context, task *Task, assigneesChanged bool) (*Task, error) {
			if !assigneesChanged {
				t.Fatal("expected assignees changed")
			}
			if len(task.AssigneeIDs) != 0 {
				t.Fatalf("expected no assignees, got %d", len(task.AssigneeIDs))
			}
			if task.Status != StatusDone {
				t.Fatalf("unexpected status: %s", task.Status)
			}
			return task, nil
		},
	}}

	_, err := svc.Update(context.Background(), taskID, actorID, PatchTaskInput{Status: &status, AssigneesSet: true, AssigneeIDs: nil})
	if err != nil {
		t.Fatalf("update task: %v", err)
	}
}

func TestDeleteTaskSuccess(t *testing.T) {
	taskID := uuid.New()
	householdID := uuid.New()
	actorID := uuid.New()
	deleted := false

	svc := &Service{Repo: &repoStub{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*Task, error) {
			return &Task{ID: taskID, HouseholdID: householdID}, nil
		},
		isHouseholdMemberFn: func(_ context.Context, _, gotUserID uuid.UUID) (bool, error) {
			return gotUserID == actorID, nil
		},
		deleteFn: func(_ context.Context, gotTaskID uuid.UUID) (bool, error) {
			if gotTaskID != taskID {
				t.Fatalf("unexpected task id: %s", gotTaskID)
			}
			deleted = true
			return true, nil
		},
	}}

	if err := svc.Delete(context.Background(), taskID, actorID); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	if !deleted {
		t.Fatal("expected delete to be called")
	}
}
