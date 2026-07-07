package app

import (
	"context"
	"fmt"

	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository/memory"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository/postgres/core"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository/postgres/demo"

	"github.com/InTacht/xqua-go/examples/showcase/pkg/services/auth"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/services/item"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/services/user"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WireDeps are the backend clients used to construct repositories and services.
type WireDeps struct {
	Core *pgxpool.Pool
	Demo *pgxpool.Pool
}

// Services holds application use cases wired from repositories.
type Services struct {
	Users *user.Service
	Items *item.Service
	Auth  *auth.Service
	Ping  func(context.Context) error
}

// Wire builds the repository facade and application services.
func Wire(deps WireDeps) (*Services, error) {
	repo, err := wireRepo(deps)
	if err != nil {
		return nil, err
	}
	return &Services{
		Users: user.NewService(repo),
		Items: item.NewService(repo),
		Auth:  auth.NewService(repo),
		Ping:  repo.Ping,
	}, nil
}

func wireRepo(deps WireDeps) (*repository.Repo, error) {
	if deps.Core == nil {
		return nil, fmt.Errorf("core postgres pool is required")
	}
	if deps.Demo == nil {
		return nil, fmt.Errorf("demo postgres pool is required")
	}

	repo := &repository.Repo{
		Users:  core.NewUsers(deps.Core),
		Items:  memory.NewItems(),
		Tokens: memory.NewKeys(),
		Audit:  demo.NewAudit(deps.Demo),
	}
	repo.SetPing(func(ctx context.Context) error {
		if err := deps.Core.Ping(ctx); err != nil {
			return fmt.Errorf("core db: %w", err)
		}
		if err := deps.Demo.Ping(ctx); err != nil {
			return fmt.Errorf("demo db: %w", err)
		}
		return nil
	})
	return repo, nil
}
