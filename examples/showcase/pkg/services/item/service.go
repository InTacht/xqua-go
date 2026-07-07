package item

import (
	"context"
	"strconv"
	"strings"

	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository"
	"github.com/InTacht/xqua-go/pkg/http/openapi"
)

// Service implements demo item use cases.
type Service struct {
	repo *repository.Repo
}

// NewService creates an item service backed by repo.
func NewService(repo *repository.Repo) *Service {
	return &Service{repo: repo}
}

// Get returns one demo item by id.
func (s *Service) Get(ctx context.Context, id int64) (*domain.Item, error) {
	return s.repo.Items.Get(ctx, id)
}

// List returns all demo items.
func (s *Service) List(ctx context.Context) ([]domain.Item, error) {
	return s.repo.Items.List(ctx)
}

// ListCursor returns items after an optional cursor token.
func (s *Service) ListCursor(ctx context.Context, cursor string, limit int) ([]domain.Item, openapi.Cursor, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	items, err := s.repo.Items.List(ctx)
	if err != nil {
		return nil, openapi.Cursor{}, err
	}
	start := int64(0)
	if cursor != "" {
		n, err := strconv.ParseInt(cursor, 10, 64)
		if err == nil {
			start = n
		}
	}
	var page []domain.Item
	for _, item := range items {
		if item.ID <= start {
			continue
		}
		page = append(page, item)
		if len(page) >= limit {
			break
		}
	}
	var next openapi.Cursor
	if len(page) == limit {
		next.Next = strconv.FormatInt(page[len(page)-1].ID, 10)
	}
	return page, next, nil
}

// Create inserts a demo item.
func (s *Service) Create(ctx context.Context, name string) (*domain.Item, error) {
	return s.repo.Items.Create(ctx, name)
}

// Update changes a demo item name.
func (s *Service) Update(ctx context.Context, id int64, name string) (*domain.Item, error) {
	return s.repo.Items.Update(ctx, id, name)
}

// Delete removes a demo item.
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Items.Delete(ctx, id)
}

// Search returns items whose name contains q (case-insensitive).
func (s *Service) Search(ctx context.Context, q string) ([]domain.Item, error) {
	items, err := s.repo.Items.List(ctx)
	if err != nil {
		return nil, err
	}
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return items, nil
	}
	var out []domain.Item
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), q) {
			out = append(out, item)
		}
	}
	return out, nil
}
