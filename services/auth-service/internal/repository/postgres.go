// Package repository stores users in PostgreSQL.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/service"
)

var _ service.UserRepository = (*UserRepository)(nil)

const uniqueViolation = "23505" // Postgres error code

const userColumns = `id, email, name, password_hash, created_at, updated_at`

// UserRepository is the Postgres implementation.
type UserRepository struct{ db *pgxpool.Pool }

// NewUserRepository wraps a pool.
func NewUserRepository(db *pgxpool.Pool) *UserRepository { return &UserRepository{db: db} }

// Create inserts a user; a duplicate email returns model.ErrEmailTaken.
func (r *UserRepository) Create(ctx context.Context, u *model.User) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO users (`+userColumns+`) VALUES ($1, $2, $3, $4, $5, $6)`,
		u.ID, u.Email, u.Name, u.PasswordHash, u.CreatedAt, u.UpdatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return model.ErrEmailTaken // the DB's UNIQUE constraint is the source of truth
	}
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// GetByEmail finds a user by (normalised) email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	return r.getOne(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)
}

// GetByID finds a user by ID.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	return r.getOne(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
}

func (r *UserRepository) getOne(ctx context.Context, query string, arg any) (*model.User, error) {
	var u model.User
	err := r.db.QueryRow(ctx, query, arg).
		Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, model.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	u.CreatedAt, u.UpdatedAt = u.CreatedAt.UTC(), u.UpdatedAt.UTC()
	return &u, nil
}
