package users

import "heimly.space/backend/internal/httpdto"

type RegisterRequest struct {
	Login    string        `json:"login"`
	Email    string        `json:"email"`
	Name     string        `json:"name"`
	Password string        `json:"password"`
	Birthday *httpdto.Date `json:"birthday,omitempty"`
}

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"access_token"`
}

type UserResponse struct {
	ID       string        `json:"id"`
	Login    string        `json:"login"`
	Email    string        `json:"email"`
	Name     string        `json:"name"`
	Birthday *httpdto.Date `json:"birthday,omitempty"`
}
