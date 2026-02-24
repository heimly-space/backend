package households

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	domain "heimly.space/backend/internal/domain/households"
	httpmw "heimly.space/backend/internal/transport/http/middleware"
)

type Handlers struct {
	Households *domain.Service
}

func (h *Handlers) ListByUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	limit := 0
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		if parsed <= 0 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	result, err := h.Households.ListByUser(r.Context(), userID, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCursor):
			http.Error(w, "invalid cursor", http.StatusBadRequest)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	resp := HouseholdsResponse{
		Items:      make([]HouseholdWithRoleResponse, 0, len(result.Items)),
		NextCursor: result.NextCursor,
	}
	for _, item := range result.Items {
		resp.Items = append(resp.Items, HouseholdWithRoleResponse{
			ID:        item.ID.String(),
			Name:      item.Name,
			Role:      item.Role,
			CreatedAt: item.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req CreateRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	household, err := h.Households.Create(r.Context(), userID, req.Name)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrHouseholdNameRequired):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusCreated, HouseholdResponse{
		ID:        household.ID.String(),
		Name:      household.Name,
		CreatedAt: household.CreatedAt,
	})
}

func (h *Handlers) InviteMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	householdID, ok := parseHouseholdID(w, r)
	if !ok {
		return
	}

	var req InviteMemberRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	member, err := h.Households.InviteMember(r.Context(), householdID, userID, req.Email)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrHouseholdNotFound):
			http.Error(w, "household not found", http.StatusNotFound)
		case errors.Is(err, domain.ErrForbidden):
			http.Error(w, "forbidden", http.StatusForbidden)
		case errors.Is(err, domain.ErrUserNotFound):
			http.Error(w, "user not found", http.StatusNotFound)
		case errors.Is(err, domain.ErrMemberAlreadyExists):
			http.Error(w, "member already exists", http.StatusConflict)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusCreated, MemberResponse{
		UserID:    member.UserID.String(),
		Email:     member.Email,
		Name:      member.Name,
		Role:      member.Role,
		CreatedAt: member.CreatedAt,
	})
}

func (h *Handlers) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	householdID, ok := parseHouseholdID(w, r)
	if !ok {
		return
	}

	members, err := h.Households.ListMembers(r.Context(), householdID, userID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrHouseholdNotFound):
			http.Error(w, "household not found", http.StatusNotFound)
		case errors.Is(err, domain.ErrForbidden):
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	resp := MembersResponse{Members: make([]MemberResponse, 0, len(members))}
	for _, m := range members {
		resp.Members = append(resp.Members, MemberResponse{
			UserID:    m.UserID.String(),
			Email:     m.Email,
			Name:      m.Name,
			Role:      m.Role,
			CreatedAt: m.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func parseHouseholdID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		http.Error(w, "invalid household id", http.StatusBadRequest)
		return uuid.Nil, false
	}
	return id, true
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
