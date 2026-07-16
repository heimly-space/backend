package tasks

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	domain "heimly.space/backend/internal/domain/tasks"
	"heimly.space/backend/internal/httpdto"
	httpmw "heimly.space/backend/internal/transport/http/middleware"
)

type Handlers struct {
	Tasks *domain.Service
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	householdID, ok := parseHouseholdID(w, r)
	if !ok {
		return
	}

	var req CreateTaskRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	input, ok := buildCreateInput(w, req)
	if !ok {
		return
	}

	task, err := h.Tasks.Create(r.Context(), householdID, userID, input)
	if err != nil {
		handleTaskError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toTaskResponse(task))
}

func (h *Handlers) ListByHousehold(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	householdID, ok := parseHouseholdID(w, r)
	if !ok {
		return
	}

	filter, ok := parseListFilter(w, r)
	if !ok {
		return
	}

	limit, ok := parseLimitQuery(w, r)
	if !ok {
		return
	}

	result, err := h.Tasks.ListByHousehold(
		r.Context(),
		householdID,
		userID,
		filter,
		r.URL.Query().Get("cursor"),
		limit,
	)
	if err != nil {
		handleTaskError(w, err)
		return
	}

	resp := ListTasksResponse{
		Items:      make([]TaskResponse, 0, len(result.Items)),
		NextCursor: result.NextCursor,
	}
	for _, item := range result.Items {
		resp.Items = append(resp.Items, toTaskResponse(&item))
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	taskID, ok := parseTaskID(w, r)
	if !ok {
		return
	}

	var req PatchTaskRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	patch, ok := buildPatchInput(w, req)
	if !ok {
		return
	}

	task, err := h.Tasks.Update(r.Context(), taskID, userID, patch)
	if err != nil {
		handleTaskError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toTaskResponse(task))
}

func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	taskID, ok := parseTaskID(w, r)
	if !ok {
		return
	}

	if err := h.Tasks.Delete(r.Context(), taskID, userID); err != nil {
		handleTaskError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func buildCreateInput(w http.ResponseWriter, req CreateTaskRequest) (domain.CreateTaskInput, bool) {
	assigneeIDs, ok := parseAssigneeIDs(w, req.AssigneeIDs)
	if !ok {
		return domain.CreateTaskInput{}, false
	}

	var due time.Time
	if req.DueAt != nil {
		due = req.DueAt.Time()
	}

	description := ""
	if req.Description != nil {
		description = *req.Description
	}

	return domain.CreateTaskInput{
		Title:       req.Title,
		Description: description,
		Status:      req.Status,
		DueAt:       due,
		AssigneeIDs: assigneeIDs,
	}, true
}

func buildPatchInput(w http.ResponseWriter, req PatchTaskRequest) (domain.PatchTaskInput, bool) {
	patch := domain.PatchTaskInput{
		Title:        req.Title,
		Description:  req.Description,
		Status:       req.Status,
		DueAtSet:     req.DueAtSet,
		AssigneesSet: req.AssigneesSet,
	}

	if req.DueAtSet && req.DueAt != nil {
		due := req.DueAt.Time()
		patch.DueAt = &due
	}

	if req.AssigneesSet {
		parsed, ok := parseAssigneeIDs(w, req.AssigneeIDs)
		if !ok {
			return domain.PatchTaskInput{}, false
		}
		patch.AssigneeIDs = parsed
	}

	return patch, true
}

func parseAssigneeIDs(w http.ResponseWriter, rawIDs []string) ([]uuid.UUID, bool) {
	if len(rawIDs) == 0 {
		return nil, true
	}

	result := make([]uuid.UUID, 0, len(rawIDs))
	for _, raw := range rawIDs {
		parsed, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			http.Error(w, "invalid assignee", http.StatusBadRequest)
			return nil, false
		}
		result = append(result, parsed)
	}
	return result, true
}

func parseListFilter(w http.ResponseWriter, r *http.Request) (domain.ListFilter, bool) {
	filter := domain.ListFilter{Status: r.URL.Query().Get("status")}

	rawAssignee := strings.TrimSpace(r.URL.Query().Get("assignee"))
	if rawAssignee == "" {
		return filter, true
	}

	parsed, err := uuid.Parse(rawAssignee)
	if err != nil {
		http.Error(w, "invalid assignee", http.StatusBadRequest)
		return domain.ListFilter{}, false
	}
	filter.AssigneeID = &parsed

	return filter, true
}

func parseLimitQuery(w http.ResponseWriter, r *http.Request) (int, bool) {
	limit := 0
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}

func parseHouseholdID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")
	parsed, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "invalid household id", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return parsed, true
}

func parseTaskID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")
	parsed, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "invalid task id", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return parsed, true
}

func handleTaskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrHouseholdNotFound):
		http.Error(w, "household not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrTaskNotFound):
		http.Error(w, "task not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrForbidden):
		http.Error(w, "forbidden", http.StatusForbidden)
	case errors.Is(err, domain.ErrTaskTitleRequired):
		http.Error(w, domain.ErrTaskTitleRequired.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrTaskStatusInvalid):
		http.Error(w, domain.ErrTaskStatusInvalid.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrTaskPatchEmpty):
		http.Error(w, domain.ErrTaskPatchEmpty.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrAssigneeNotInHousehold):
		http.Error(w, domain.ErrAssigneeNotInHousehold.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidCursor):
		http.Error(w, "invalid cursor", http.StatusBadRequest)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func toTaskResponse(task *domain.Task) TaskResponse {
	resp := TaskResponse{
		ID:          task.ID.String(),
		HouseholdID: task.HouseholdID.String(),
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}
	if !task.DueAt.IsZero() {
		d := httpdto.Date(task.DueAt)
		resp.DueAt = &d
	}
	if len(task.AssigneeIDs) > 0 {
		resp.AssigneeIDs = make([]string, 0, len(task.AssigneeIDs))
		for _, assigneeID := range task.AssigneeIDs {
			resp.AssigneeIDs = append(resp.AssigneeIDs, assigneeID.String())
		}
	}
	return resp
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}

	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}

	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
	_, _ = w.Write([]byte("\n"))
}
