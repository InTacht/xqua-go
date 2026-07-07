package repository

import (
	"context"

	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
)

// UserRepository loads persisted users from the core database.
type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*domain.User, error)
	List(ctx context.Context, limit int) ([]domain.User, error)
	ListPaged(ctx context.Context, page, size int) ([]domain.User, int, error)
	Update(ctx context.Context, id int64, name, email string) (*domain.User, error)
}

// ItemRepository loads demo catalog items from ephemeral storage.
type ItemRepository interface {
	Get(ctx context.Context, id int64) (*domain.Item, error)
	List(ctx context.Context) ([]domain.Item, error)
	Create(ctx context.Context, name string) (*domain.Item, error)
	Update(ctx context.Context, id int64, name string) (*domain.Item, error)
	Delete(ctx context.Context, id int64) error
}

// TokenRepository issues and resolves demo API keys from ephemeral storage.
type TokenRepository interface {
	Issue(ctx context.Context, username string) (token string, session domain.Session, err error)
	Lookup(ctx context.Context, raw string) (domain.Session, bool)
}

// AuditRepository loads user activity from the demo database.
type AuditRepository interface {
	ListByUser(ctx context.Context, userID int64, limit int) ([]domain.AuditEntry, error)
}
