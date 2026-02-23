package users

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	domain "heimly.space/backend/internal/domain/users"
	"heimly.space/backend/internal/httpdto"
	httpmw "heimly.space/backend/internal/transport/http/middleware"
)

type AuthHandlers struct {
	Users *domain.Service
}

func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	birthday := time.Time{}
	if req.Birthday != nil {
		birthday = req.Birthday.Time()
	}

	token, err := h.Users.Register(
		r.Context(),
		req.Login,
		req.Email,
		req.Name,
		req.Password,
		birthday,
	)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUserExists):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{Token: token})
}

func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	token, err := h.Users.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCredentials):
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{Token: token})
}

func (h *AuthHandlers) GetMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.Users.GetByID(r.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrUserNotFound):
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, http.StatusOK, UserResponse{
		ID:       user.ID.String(),
		Login:    user.Login,
		Email:    user.Email,
		Name:     user.Name,
		Birthday: birthdayPtr(user.Birthday),
	})
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

func birthdayPtr(t time.Time) *httpdto.Date {
	if t.IsZero() {
		return nil
	}
	d := httpdto.Date(t)
	return &d
}
