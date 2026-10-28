package repository

import (
	"context"
	"errors"
	"fmt"
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

const userColumns = `id, email, password_hash, name, role, email_verified, created_at, updated_at`

func scanUser(row pgx.Row) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.Role, &u.EmailVerified, &u.CreatedAt, &u.UpdatedAt)
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

// List returns one page of the admin user directory plus the total
// matching the same filters.
func (r *Postgres) List(ctx context.Context, f model.ListFilter) ([]model.User, int64, error) {
	where, args := userWhere(f)

	countQ := `SELECT COUNT(*) FROM users WHERE ` + where
	var total int64
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, apperr.Internal(err)
	}

	listArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	q := `SELECT ` + userColumns + ` FROM users WHERE ` + where +
		` ORDER BY ` + userOrder(f.Sort) +
		fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, q, listArgs...)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.User{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return out, total, nil
}

func userWhere(f model.ListFilter) (string, []any) {
	clauses := []string{"1 = 1"}
	args := []any{}
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if f.Query != "" {
		pattern := "%" + escapeLike(f.Query) + "%"
		n := add(pattern)
		clauses = append(clauses, fmt.Sprintf(
			`(lower(email) LIKE lower(%s) ESCAPE '\' OR lower(name) LIKE lower(%s) ESCAPE '\')`,
			n, n))
	}
	if f.Role != "" {
		clauses = append(clauses, fmt.Sprintf("role = %s", add(f.Role)))
	}
	if f.EmailVerified != nil {
		clauses = append(clauses, fmt.Sprintf("email_verified = %s", add(*f.EmailVerified)))
	}
	if f.CreatedAfter != nil {
		clauses = append(clauses, fmt.Sprintf("created_at >= %s", add(*f.CreatedAfter)))
	}
	if f.CreatedBefore != nil {
		clauses = append(clauses, fmt.Sprintf("created_at <= %s", add(*f.CreatedBefore)))
	}
	return strings.Join(clauses, " AND "), args
}

func userOrder(sortKey string) string {
	switch sortKey {
	case "oldest":
		return "created_at ASC, id ASC"
	case "name_asc":
		return "name ASC, id ASC"
	case "name_desc":
		return "name DESC, id ASC"
	case "email_asc":
		return "email ASC, id ASC"
	case "email_desc":
		return "email DESC, id ASC"
	default: // newest
		return "created_at DESC, id ASC"
	}
}

// escapeLike makes user input literal inside an ILIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *Postgres) SetRole(ctx context.Context, id, role string) (model.User, error) {
	const q = `UPDATE users SET role = $2, updated_at = now() WHERE id = $1
	           RETURNING ` + userColumns
	return scanUser(r.pool.QueryRow(ctx, q, id, role))
}

// SetEmailVerified flips the verification flag (auth's OTP/reset flows).
func (r *Postgres) SetEmailVerified(ctx context.Context, id string, verified bool) (model.User, error) {
	const q = `UPDATE users SET email_verified = $2, updated_at = now() WHERE id = $1
	           RETURNING ` + userColumns
	return scanUser(r.pool.QueryRow(ctx, q, id, verified))
}

// SetPasswordHash replaces the stored hash (auth's reset flow).
func (r *Postgres) SetPasswordHash(ctx context.Context, id, hash string) (model.User, error) {
	const q = `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1
	           RETURNING ` + userColumns
	return scanUser(r.pool.QueryRow(ctx, q, id, hash))
}
