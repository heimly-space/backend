package users

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID       uuid.UUID
	Login    string
	Email    string
	Name     string
	Birthday time.Time
}

type UserWithPassword struct {
	User
	HashedPassword string
}
