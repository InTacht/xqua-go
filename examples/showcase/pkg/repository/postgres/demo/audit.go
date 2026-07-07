package demo

import (
	"context"

	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository"

	"github.com/InTacht/xqua-go/pkg/errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Audit is the Postgres implementation of repository.AuditRepository.
type Audit struct {
	pool *pgxpool.Pool
}

// NewAudit creates a demo-database audit repository.
func NewAudit(pool *pgxpool.Pool) repository.AuditRepository {
	return &Audit{pool: pool}
}

// ListByUser returns recent audit entries for a user ordered newest first.
func (r *Audit) ListByUser(ctx context.Context, userID int64, limit int) ([]domain.AuditEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, action, created_at
		FROM audit_log
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, errors.Wrap(err, ErrQuery)
	}
	defer rows.Close()

	var entries []domain.AuditEntry
	for rows.Next() {
		var entry domain.AuditEntry
		if err := rows.Scan(&entry.ID, &entry.UserID, &entry.Action, &entry.CreatedAt); err != nil {
			return nil, errors.Wrap(err, ErrQuery)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, ErrQuery)
	}
	return entries, nil
}
