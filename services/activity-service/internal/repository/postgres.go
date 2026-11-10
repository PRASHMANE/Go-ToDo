// Package repository stores activities in PostgreSQL.
package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/service"
)

var _ service.ActivityRepository = (*ActivityRepository)(nil)

// ActivityRepository is the Postgres implementation.
type ActivityRepository struct{ db *pgxpool.Pool }

// NewActivityRepository wraps a pool.
func NewActivityRepository(db *pgxpool.Pool) *ActivityRepository {
	return &ActivityRepository{db: db}
}

// Insert stores an activity. Returns inserted=false for a duplicate event.
func (r *ActivityRepository) Insert(ctx context.Context, a *model.Activity) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`INSERT INTO activities (id, event_id, user_id, todo_id, type, title, occurred_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (event_id) DO NOTHING`,
		a.ID, a.EventID, a.UserID, a.TodoID, a.Type, a.Title, a.OccurredAt)
	if err != nil {
		return false, fmt.Errorf("insert activity: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ListByUser returns the latest activities for a user.
func (r *ActivityRepository) ListByUser(ctx context.Context, userID string, limit int) ([]model.Activity, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, event_id, user_id, todo_id, type, title, occurred_at
		   FROM activities WHERE user_id = $1 ORDER BY occurred_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list activities: %w", err)
	}
	// pgx.CollectRows + RowToStructByPos maps columns to fields in order.
	items, err := pgx.CollectRows(rows, pgx.RowToStructByPos[model.Activity])
	if err != nil {
		return nil, fmt.Errorf("collect activities: %w", err)
	}
	for i := range items {
		items[i].OccurredAt = items[i].OccurredAt.UTC()
	}
	return items, nil
}
