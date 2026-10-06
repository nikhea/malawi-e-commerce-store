package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/products/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the pgx-backed products store. DB access only.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

// CategoryName comes from a LEFT JOIN so deleted categories read as "".
const productColumns = `p.id, p.category_id, c.name AS category_name, p.name, p.slug, p.description, p.price_cents, p.currency, p.image_url, p.image_public_id, p.is_active, p.created_at, p.updated_at`
const productFrom = `FROM products p LEFT JOIN categories c ON c.id = p.category_id`

func scanProduct(row pgx.Row) (model.Product, error) {
	var p model.Product
	var categoryName *string
	err := row.Scan(&p.ID, &p.CategoryID, &categoryName, &p.Name, &p.Slug, &p.Description,
		&p.PriceCents, &p.Currency, &p.ImageURL, &p.ImagePublicID, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Product{}, apperr.NotFound("product not found")
	}
	if err != nil {
		return model.Product{}, apperr.Internal(err)
	}
	if categoryName != nil {
		p.CategoryName = *categoryName
	}
	return p, nil
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func (r *Postgres) Create(ctx context.Context, p model.Product) (model.Product, error) {
	// Insert, then re-read for the category JOIN. Two round trips keep
	// each query simple and race-free.
	created, err := r.createRow(ctx, p)
	if err != nil {
		return model.Product{}, err
	}
	return r.GetByID(ctx, created.ID)
}

func (r *Postgres) createRow(ctx context.Context, p model.Product) (model.Product, error) {
	const q = `INSERT INTO products (category_id, name, slug, description, price_cents, currency, image_url)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)
	           RETURNING id`
	var id string
	err := r.pool.QueryRow(ctx, q, p.CategoryID, p.Name, p.Slug, p.Description,
		p.PriceCents, p.Currency, p.ImageURL).Scan(&id)
	if err != nil {
		switch pgCode(err) {
		case "23505":
			return model.Product{}, apperr.Conflict("slug already in use")
		case "23503":
			return model.Product{}, apperr.NotFound("category not found")
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Product{}, apperr.Internal(fmt.Errorf("insert returned no id"))
		}
		return model.Product{}, apperr.Internal(err)
	}
	p.ID = id
	return p, nil
}

func (r *Postgres) GetByID(ctx context.Context, id string) (model.Product, error) {
	const q = `SELECT ` + productColumns + ` ` + productFrom + ` WHERE p.id = $1`
	return scanProduct(r.pool.QueryRow(ctx, q, id))
}

func (r *Postgres) GetBySlug(ctx context.Context, slug string) (model.Product, error) {
	const q = `SELECT ` + productColumns + ` ` + productFrom + ` WHERE p.slug = $1`
	return scanProduct(r.pool.QueryRow(ctx, q, strings.ToLower(slug)))
}

func (r *Postgres) List(ctx context.Context, categoryID *string, activeOnly bool, limit, offset int) ([]model.Product, error) {
	q := `SELECT ` + productColumns + ` ` + productFrom + ` WHERE 1 = 1`
	args := []any{}
	if activeOnly {
		q += ` AND p.is_active`
	}
	if categoryID != nil {
		args = append(args, *categoryID)
		q += fmt.Sprintf(` AND p.category_id = $%d`, len(args))
	}
	args = append(args, limit, offset)
	q += fmt.Sprintf(` ORDER BY p.name LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	return out, nil
}

func (r *Postgres) Update(ctx context.Context, p model.Product) (model.Product, error) {
	const q = `UPDATE products
	           SET category_id = $2, name = $3, description = $4, price_cents = $5,
	               currency = $6, is_active = $7, updated_at = now()
	           WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, p.ID, p.CategoryID, p.Name, p.Description,
		p.PriceCents, p.Currency, p.IsActive)
	if err != nil {
		if pgCode(err) == "23503" {
			return model.Product{}, apperr.NotFound("category not found")
		}
		return model.Product{}, apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return model.Product{}, apperr.NotFound("product not found")
	}
	return r.GetByID(ctx, p.ID)
}

func (r *Postgres) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM products WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("product not found")
	}
	return nil
}

func (r *Postgres) SetImage(ctx context.Context, id, url, publicID string) (model.Product, error) {
	const q = `UPDATE products
	           SET image_url = $2, image_public_id = $3, updated_at = now()
	           WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id, url, publicID)
	if err != nil {
		return model.Product{}, apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return model.Product{}, apperr.NotFound("product not found")
	}
	return r.GetByID(ctx, id)
}
