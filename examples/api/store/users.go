package store

import (
	"context"
	stderrors "errors"

	"github.com/InTacht/xqua-go/pkg/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User is a row from the users table.
type User struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Users reads and writes user records.
type Users struct {
	pool *pgxpool.Pool
}

// NewUsers creates a user store backed by pool.
func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

// GetByID returns one user or ErrNotFound.
func (s *Users) GetByID(ctx context.Context, id int64) (*User, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, email
		FROM users
		WHERE id = $1
	`, id)

	var user User
	if err := row.Scan(&user.ID, &user.Name, &user.Email); err != nil {
		if stderrors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, errors.Wrap(err, ErrQuery)
	}
	return &user, nil
}

// List returns up to limit users ordered by id.
func (s *Users) List(ctx context.Context, limit int) ([]User, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, name, email
		FROM users
		ORDER BY id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, errors.Wrap(err, ErrQuery)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
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
