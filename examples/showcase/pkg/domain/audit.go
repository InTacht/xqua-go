package domain

import "time"

// AuditEntry is an append-only activity record stored in the demo Postgres database.
type AuditEntry struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
}
