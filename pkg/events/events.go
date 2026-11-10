// Package events defines the messages exchanged between services.
// It is the CONTRACT between publisher and consumers: only ever ADD fields.
package events

import "time"

// StreamName is the JetStream stream holding all todo events.
const StreamName = "TODOS"

// Subjects.
const (
	SubjectTodoCreated   = "todos.created"
	SubjectTodoUpdated   = "todos.updated"
	SubjectTodoCompleted = "todos.completed"
	SubjectTodoDeleted   = "todos.deleted"
	SubjectTodoAll       = "todos.>"
)

// TodoEvent is published whenever a todo changes.
type TodoEvent struct {
	EventID    string    `json:"event_id"` // unique → idempotency + dedupe
	Type       string    `json:"type"`     // same as the subject
	OccurredAt time.Time `json:"occurred_at"`
	UserID     string    `json:"user_id"`
	TodoID     string    `json:"todo_id"`
	Title      string    `json:"title"`
}
