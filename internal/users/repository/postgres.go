package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/users/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the pgx-backed users store. DB access only: no business
// rules here beyond mapping driver outcomes to coded apperr errors.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

const userColumns = `id, email, password_hash, name, role, created_at, updated_at`

func scanUser(row pgx.Row) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, apperr.NotFound("user not found")
	}
	if err != nil {
		return model.User{}, apperr.Internal(err)
	}
	return u, nil
}

func (r *Postgres) Create(ctx context.Context, u model.User) (model.User, error) {
	const q = `INSERT INTO users (email, password_hash, name, role)
	           VALUES ($1, $2, $3, $4)
	           RETURNING ` + userColumns
	created, err := scanUser(r.pool.QueryRow(ctx, q,
		strings.ToLower(u.Email), u.PasswordHash, u.Name, u.Role))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.User{}, apperr.Conflict("email already registered")
		}
		// scanUser already wrapped non-constraint failures; pass through.
		return model.User{}, err
	}
	return created, nil
}

func (r *Postgres) GetByID(ctx context.Context, id string) (model.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	return scanUser(r.pool.QueryRow(ctx, q, id))
}

func (r *Postgres) GetByEmail(ctx context.Context, email string) (model.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE email = $1`
	return scanUser(r.pool.QueryRow(ctx, q, strings.ToLower(email)))
}

func (r *Postgres) SetRole(ctx context.Context, id, role string) (model.User, error) {
	const q = `UPDATE users SET role = $2, updated_at = now() WHERE id = $1
	           RETURNING ` + userColumns
	return scanUser(r.pool.QueryRow(ctx, q, id, role))
}
