// Package dto defines activity-service JSON shapes.
package dto

import (
	"time"

	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/model"
)

// ListActivityQuery is the query string of GET /activity.
type ListActivityQuery struct {
	Limit int `form:"limit,default=20" binding:"min=1,max=100"`
}

// ActivityResponse is one activity as JSON.
type ActivityResponse struct {
	ID         string    `json:"id"`
	TodoID     string    `json:"todo_id"`
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	OccurredAt time.Time `json:"occurred_at"`
}

// ListActivityResponse wraps the feed.
type ListActivityResponse struct {
	Items []ActivityResponse `json:"items"`
}

// FromModel maps a domain Activity to JSON.
func FromModel(a *model.Activity) ActivityResponse {
	return ActivityResponse{ID: a.ID, TodoID: a.TodoID, Type: a.Type, Title: a.Title, OccurredAt: a.OccurredAt}
}
