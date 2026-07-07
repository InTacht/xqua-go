package core

import (
	"context"
	stderrors "errors"

	"github.com/InTacht/xqua-go/examples/showcase/pkg/domain"
	"github.com/InTacht/xqua-go/examples/showcase/pkg/repository"

	"github.com/InTacht/xqua-go/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Users is the Postgres implementation of repository.UserRepository.
type Users struct {
	pool *pgxpool.Pool
}

// NewUsers creates a core-database user repository.
func NewUsers(pool *pgxpool.Pool) repository.UserRepository {
	return &Users{pool: pool}
}

// GetByID returns one user or ErrNotFound.
func (r *Users) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, name, email
		FROM users
		WHERE id = $1
	`, id)

	var user domain.User
	if err := row.Scan(&user.ID, &user.Name, &user.Email); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, errors.Wrap(err, ErrQuery)
	}
	return &user, nil
}

// List returns up to limit users ordered by id.
func (r *Users) List(ctx context.Context, limit int) ([]domain.User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, email
		FROM users
		ORDER BY id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, errors.Wrap(err, ErrQuery)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
			return nil, errors.Wrap(err, ErrQuery)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, ErrQuery)
	}
	return users, nil
}

// ListPaged returns one page of users and the total row count.
func (r *Users) ListPaged(ctx context.Context, page, size int) ([]domain.User, int, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, errors.Wrap(err, ErrQuery)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, name, email
		FROM users
		ORDER BY id
		LIMIT $1 OFFSET $2
	`, size, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, ErrQuery)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
			return nil, 0, errors.Wrap(err, ErrQuery)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, errors.Wrap(err, ErrQuery)
	}
	return users, total, nil
}

// Update changes name and email for one user.
func (r *Users) Update(ctx context.Context, id int64, name, email string) (*domain.User, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE users
		SET name = $2, email = $3
		WHERE id = $1
		RETURNING id, name, email
	`, id, name, email)

	var user domain.User
	if err := row.Scan(&user.ID, &user.Name, &user.Email); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if isUniqueViolation(err) {
			return nil, ErrConflict
		}
		return nil, errors.Wrap(err, ErrQuery)
	}
	return &user, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return stderrors.As(err, &pgErr) && pgErr.Code == "23505"
}
