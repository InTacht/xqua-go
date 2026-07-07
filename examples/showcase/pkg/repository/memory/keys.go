package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository"

	"github.com/InTacht/xqua-go/pkg/errors"
)

// Keys is the in-memory implementation of repository.TokenRepository.
type Keys struct {
	mu   sync.RWMutex
	keys map[string]domain.Session
	next int64
}

// NewKeys creates an empty demo token repository.
func NewKeys() repository.TokenRepository {
	return &Keys{keys: map[string]domain.Session{}}
}

// Issue creates a new API key for username and returns the raw secret once.
func (k *Keys) Issue(_ context.Context, username string) (string, domain.Session, error) {
	raw, err := randomToken()
	if err != nil {
		return "", domain.Session{}, errors.Wrap(err, ErrIssueToken)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	k.next++
	session := domain.Session{ID: k.next, Username: username}
	k.keys[raw] = session
	return raw, session, nil
}

// Lookup returns the session for a raw API key.
func (k *Keys) Lookup(_ context.Context, raw string) (domain.Session, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	session, ok := k.keys[raw]
	return session, ok
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("random token: %w", err)
	}
	return "sk_demo_" + hex.EncodeToString(b), nil
}
