package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/categories/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the pgx-backed categories store. DB access only.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

const categoryColumns = `id, name, slug, description, parent_id, image_url, image_public_id, created_at, updated_at`

func scanCategory(row pgx.Row) (model.Category, error) {
	var c model.Category
	err := row.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.ParentID, &c.ImageURL, &c.ImagePublicID, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Category{}, apperr.NotFound("category not found")
	}
	if err != nil {
		return model.Category{}, apperr.Internal(err)
	}
	return c, nil
}

func isConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *Postgres) Create(ctx context.Context, c model.Category) (model.Category, error) {
	const q = `INSERT INTO categories (name, slug, description, parent_id, image_url)
	           VALUES ($1, $2, $3, $4, $5)
	           RETURNING ` + categoryColumns
	created, err := scanCategory(r.pool.QueryRow(ctx, q, c.Name, c.Slug, c.Description, c.ParentID, c.ImageURL))
	if err != nil {
		if isConflict(err) {
			return model.Category{}, apperr.Conflict("slug already in use")
		}
		return model.Category{}, err
	}
	return created, nil
}

func (r *Postgres) GetByID(ctx context.Context, id string) (model.Category, error) {
	const q = `SELECT ` + categoryColumns + ` FROM categories WHERE id = $1`
	return scanCategory(r.pool.QueryRow(ctx, q, id))
}

func (r *Postgres) GetBySlug(ctx context.Context, slug string) (model.Category, error) {
	const q = `SELECT ` + categoryColumns + ` FROM categories WHERE slug = $1`
	return scanCategory(r.pool.QueryRow(ctx, q, strings.ToLower(slug)))
}

func (r *Postgres) List(ctx context.Context) ([]model.Category, error) {
	const q = `SELECT ` + categoryColumns + ` FROM categories ORDER BY name`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Category{}
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	return out, nil
}

func (r *Postgres) Update(ctx context.Context, c model.Category) (model.Category, error) {
	const q = `UPDATE categories
	           SET name = $2, description = $3, parent_id = $4, updated_at = now()
	           WHERE id = $1
	           RETURNING ` + categoryColumns
	updated, err := scanCategory(r.pool.QueryRow(ctx, q, c.ID, c.Name, c.Description, c.ParentID))
	if err != nil {
		return model.Category{}, err
	}
	return updated, nil
}

func (r *Postgres) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM categories WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("category not found")
	}
	return nil
}

// SetImage stores the finished upload outcome (direct link or pipeline
// callback — both funnel here).
func (r *Postgres) SetImage(ctx context.Context, id, url, publicID string) (model.Category, error) {
	const q = `UPDATE categories
	           SET image_url = $2, image_public_id = $3, updated_at = now()
	           WHERE id = $1
	           RETURNING ` + categoryColumns
	return scanCategory(r.pool.QueryRow(ctx, q, id, url, publicID))
}
