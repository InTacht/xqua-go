package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository"
)

// Items is the in-memory implementation of repository.ItemRepository.
type Items struct {
	mu     sync.RWMutex
	data   map[int64]string
	nextID int64
}

// NewItems creates a demo item repository seeded with sample rows.
func NewItems() repository.ItemRepository {
	return &Items{
		data: map[int64]string{
			1: "alpha",
			2: "beta",
		},
		nextID: 3,
	}
}

// Get returns one item or an internal repository error.
func (r *Items) Get(_ context.Context, id int64) (*domain.Item, error) {
	if id == 99 {
		return nil, ErrItemCorrupt
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	name, ok := r.data[id]
	if !ok {
		return nil, ErrItemNotFound
	}
	return &domain.Item{ID: id, Name: name}, nil
}

// List returns all items ordered by id.
func (r *Items) List(_ context.Context) ([]domain.Item, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]int64, 0, len(r.data))
	for id := range r.data {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	items := make([]domain.Item, 0, len(ids))
	for _, id := range ids {
		items = append(items, domain.Item{ID: id, Name: r.data[id]})
	}
	return items, nil
}

// Create inserts a new demo item.
func (r *Items) Create(_ context.Context, name string) (*domain.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := r.nextID
	r.nextID++
	r.data[id] = name
	return &domain.Item{ID: id, Name: name}, nil
}

// Update changes one item name.
func (r *Items) Update(_ context.Context, id int64, name string) (*domain.Item, error) {
	if id == 99 {
		return nil, ErrItemCorrupt
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.data[id]; !ok {
		return nil, ErrItemNotFound
	}
	r.data[id] = name
	return &domain.Item{ID: id, Name: name}, nil
}

// Delete removes one item.
func (r *Items) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.data[id]; !ok {
		return ErrItemNotFound
	}
	delete(r.data, id)
	return nil
}
