// Package dto defines the JSON request/response shapes of the HTTP API.
package dto

import (
	"time"

	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/model"
)

// CreateTodoRequest is the body of POST /todos.
type CreateTodoRequest struct {
	Title       string     `json:"title" binding:"required,min=1,max=200"`
	Description string     `json:"description" binding:"max=2000"`
	Priority    string     `json:"priority" binding:"omitempty,oneof=low medium high"`
	DueDate     *time.Time `json:"due_date"` // RFC3339, e.g. "2026-12-01T00:00:00Z"
}

// UpdateTodoRequest is the body of PATCH /todos/:id. All fields optional.
type UpdateTodoRequest struct {
	Title        *string    `json:"title" binding:"omitempty,min=1,max=200"`
	Description  *string    `json:"description" binding:"omitempty,max=2000"`
	Completed    *bool      `json:"completed"`
	Priority     *string    `json:"priority" binding:"omitempty,oneof=low medium high"`
	DueDate      *time.Time `json:"due_date"`
	ClearDueDate bool       `json:"clear_due_date"` // true = remove the due date
}

// ListTodosQuery is the query string of GET /todos.
type ListTodosQuery struct {
	Status   string `form:"status" binding:"omitempty,oneof=all active completed"`
	Priority string `form:"priority" binding:"omitempty,oneof=low medium high"`
	Q        string `form:"q" binding:"max=100"`
	Page     int    `form:"page,default=1" binding:"min=1"`
	PageSize int    `form:"page_size,default=20" binding:"min=1,max=100"`
}

// TodoResponse is how a todo looks in JSON.
type TodoResponse struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Completed   bool       `json:"completed"`
	Priority    string     `json:"priority"`
	DueDate     *time.Time `json:"due_date"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ListTodosResponse wraps a page of todos.
type ListTodosResponse struct {
	Items    []TodoResponse `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int            `json:"total"`
}

// FromModel maps a domain Todo to its JSON form.
func FromModel(t *model.Todo) TodoResponse {
	return TodoResponse{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Completed:   t.Completed,
		Priority:    string(t.Priority),
		DueDate:     t.DueDate,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}
