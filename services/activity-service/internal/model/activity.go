// Package model holds activity-service domain types.
package model

import "time"

// Activity is one entry in a user's activity feed.
type Activity struct {
	ID         string
	EventID    string
	UserID     string
	TodoID     string
	Type       string
	Title      string
	OccurredAt time.Time
}
