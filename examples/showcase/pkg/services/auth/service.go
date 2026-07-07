package auth

import (
	"context"

	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository"

	"github.com/InTacht/xqua-go/pkg/errors"
)

// ErrInvalidCredentials is returned when demo login credentials are wrong.
var ErrInvalidCredentials = errors.NewPlain("invalid credentials")

// Service implements demo auth use cases.
type Service struct {
	repo *repository.Repo
}

// NewService creates an auth service backed by repo.
func NewService(repo *repository.Repo) *Service {
	return &Service{repo: repo}
}

// Login validates demo credentials and issues a new API key.
func (s *Service) Login(ctx context.Context, username, password string) (string, domain.Session, error) {
	if username != "demo" || password != "secret" {
		return "", domain.Session{}, ErrInvalidCredentials
	}
	return s.repo.Tokens.Issue(ctx, username)
}

// Lookup resolves a raw API key to a session.
func (s *Service) Lookup(ctx context.Context, raw string) (domain.Session, bool) {
	return s.repo.Tokens.Lookup(ctx, raw)
}
