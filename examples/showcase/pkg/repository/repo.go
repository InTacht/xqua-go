package repository

import (
	"context"
	"fmt"
)

// Repo groups domain repositories wired to their backends.
//
// Core Postgres holds primary entities (users). Demo Postgres holds ancillary
// data (audit trail). Memory backends stand in for ephemeral stores (demo
// items, API tokens — Redis-like).
type Repo struct {
	Users  UserRepository
	Items  ItemRepository
	Tokens TokenRepository
	Audit  AuditRepository

	ping func(context.Context) error
}

// Ping checks connectivity for every Postgres backend.
func (r *Repo) Ping(ctx context.Context) error {
	if r.ping == nil {
		return fmt.Errorf("repository ping is not configured")
	}
	return r.ping(ctx)
}

// SetPing configures health checks for a wired repo.
func (r *Repo) SetPing(ping func(context.Context) error) {
	r.ping = ping
}
