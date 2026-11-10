// Package model holds todo-service domain types. No JSON, no SQL here.
package model

import (
	"errors"
	"time"
)

// Domain errors. Handlers map these to HTTP status codes.
var (
	ErrNotFound   = errors.New("todo not found")
	ErrValidation = errors.New("validation failed")
)

// Priority is a typed string: the compiler stops you passing a random string by accident.
type Priority string

// Allowed priorities.
const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
)

// Valid reports whether p is a known priority.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh:
		return true
	}
	return false
}

// Todo is the core domain entity.
type Todo struct {
	ID          string
	UserID      string
	Title       string
	Description string
	Completed   bool
	Priority    Priority
	DueDate     *time.Time // nil = no due date
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListFilter describes a list query. Zero values mean "no filter".
type ListFilter struct {
	UserID    string
	Completed *bool // nil = all, true = completed, false = active
	Priority  Priority
	Search    string
	Limit     int
	Offset    int
}
