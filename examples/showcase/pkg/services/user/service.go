package user

import (
	"context"
	"math"

	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
)

const defaultListLimit = 50

// Service implements user use cases.
type Service struct {
	repo *repository.Repo
}

// NewService creates a user service backed by repo.
func NewService(repo *repository.Repo) *Service {
	return &Service{repo: repo}
}

// List returns up to limit users with service-level bounds applied.
func (s *Service) List(ctx context.Context, limit int) ([]domain.User, error) {
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.Users.List(ctx, limit)
}

// ListPaged returns one page of users and pagination metadata.
func (s *Service) ListPaged(ctx context.Context, page, size int) ([]domain.User, openapi.Pagination, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	users, total, err := s.repo.Users.ListPaged(ctx, page, size)
	if err != nil {
		return nil, openapi.Pagination{}, err
	}
	totalPages := int(math.Ceil(float64(total) / float64(size)))
	if totalPages == 0 {
		totalPages = 1
	}
	return users, openapi.Pagination{
		TotalCount: total,
		TotalPages: totalPages,
		MaxPage:    totalPages,
		Page:       page,
		Size:       size,
		First:      page == 1,
		Last:       page >= totalPages,
	}, nil
}

// Get returns one user by id.
func (s *Service) Get(ctx context.Context, id int64) (*domain.User, error) {
	return s.repo.Users.GetByID(ctx, id)
}

// Update changes one user's name and email.
func (s *Service) Update(ctx context.Context, id int64, name, email string) (*domain.User, error) {
	return s.repo.Users.Update(ctx, id, name, email)
}

// ListAudit returns recent audit entries for a user from the demo database.
//
// The use case spans two Postgres backends: user existence is checked on core,
// audit rows are read from demo. There is no cross-database transaction.
func (s *Service) ListAudit(ctx context.Context, userID int64, limit int) ([]domain.AuditEntry, error) {
	if _, err := s.repo.Users.GetByID(ctx, userID); err != nil {
		return nil, err
	}
	return s.repo.Audit.ListByUser(ctx, userID, limit)
}
