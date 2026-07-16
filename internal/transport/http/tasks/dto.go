package tasks

import (
	"encoding/json"
	"fmt"
	"time"

	"heimly.space/backend/internal/httpdto"
)

type CreateTaskRequest struct {
	Title       string        `json:"title"`
	Description *string       `json:"description"`
	Status      string        `json:"status"`
	DueAt       *httpdto.Date `json:"due_at"`
	AssigneeIDs []string      `json:"assignee_ids"`
}

type PatchTaskRequest struct {
	Title        *string       `json:"-"`
	Description  *string       `json:"-"`
	Status       *string       `json:"-"`
	DueAt        *httpdto.Date `json:"-"`
	DueAtSet     bool          `json:"-"`
	AssigneeIDs  []string      `json:"-"`
	AssigneesSet bool          `json:"-"`
}

func (r *PatchTaskRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	for key, value := range raw {
		switch key {
		case "title":
			var parsed string
			if err := json.Unmarshal(value, &parsed); err != nil {
				return err
			}
			r.Title = &parsed
		case "description":
			if string(value) == "null" {
				empty := ""
				r.Description = &empty
				continue
			}
			var parsed string
			if err := json.Unmarshal(value, &parsed); err != nil {
				return err
			}
			r.Description = &parsed
		case "status":
			var parsed string
			if err := json.Unmarshal(value, &parsed); err != nil {
				return err
			}
			r.Status = &parsed
		case "due_at":
			r.DueAtSet = true
			if string(value) == "null" {
				r.DueAt = nil
				continue
			}
			var parsed httpdto.Date
			if err := json.Unmarshal(value, &parsed); err != nil {
				return err
			}
			r.DueAt = &parsed
		case "assignee_ids":
			r.AssigneesSet = true
			if string(value) == "null" {
				r.AssigneeIDs = nil
				continue
			}
			var parsed []string
			if err := json.Unmarshal(value, &parsed); err != nil {
				return err
			}
			r.AssigneeIDs = parsed
		default:
			return fmt.Errorf("unknown field %q", key)
		}
	}

	return nil
}

type TaskResponse struct {
	ID          string        `json:"id"`
	HouseholdID string        `json:"household_id"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	Status      string        `json:"status"`
	DueAt       *httpdto.Date `json:"due_at,omitempty"`
	AssigneeIDs []string      `json:"assignee_ids,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

type ListTasksResponse struct {
	Items      []TaskResponse `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}
