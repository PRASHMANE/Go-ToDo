// Package service turns todo events into an activity feed.
package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/PRASHMANE/Go-ToDo/pkg/events"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/model"
)

// ErrInvalidEvent means the event can never be processed (retrying won't help).
var ErrInvalidEvent = errors.New("invalid event: missing ids")

// ActivityRepository is what the service needs from storage.
type ActivityRepository interface {
	Insert(ctx context.Context, a *model.Activity) (bool, error)
	ListByUser(ctx context.Context, userID string, limit int) ([]model.Activity, error)
}

// ActivityService implements activity use cases.
type ActivityService struct{ repo ActivityRepository }

// NewActivityService builds the service.
func NewActivityService(repo ActivityRepository) *ActivityService {
	return &ActivityService{repo: repo}
}

// Record turns an event into an activity row. Idempotent: replaying the same
// event (same EventID) is a no-op, because delivery is at-least-once.
// It returns whether a new row was written.
func (s *ActivityService) Record(ctx context.Context, e events.TodoEvent) (bool, error) {
	if e.EventID == "" || e.UserID == "" || e.TodoID == "" {
		return false, ErrInvalidEvent
	}
	return s.repo.Insert(ctx, &model.Activity{
		ID: uuid.NewString(), EventID: e.EventID, UserID: e.UserID,
		TodoID: e.TodoID, Type: e.Type, Title: e.Title, OccurredAt: e.OccurredAt.UTC(),
	})
}

// List returns a user's latest activities.
func (s *ActivityService) List(ctx context.Context, userID string, limit int) ([]model.Activity, error) {
	return s.repo.ListByUser(ctx, userID, limit)
}
