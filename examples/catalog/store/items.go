package store

import (
	"context"
	"sync"
)

// Items is an in-memory store that surfaces internal catalog errors.
type Items struct {
	mu   sync.RWMutex
	data map[int64]string
}

func NewItems() *Items {
	return &Items{data: map[int64]string{
		1: "alpha",
		2: "beta",
	}}
}

func (s *Items) Get(_ context.Context, id int64) (string, error) {
	if id == 99 {
		// Simulate an internal failure that must be mapped or fall back.
		return "", ErrCorrupt
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[id]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}
