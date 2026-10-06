package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/variants/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the pgx-backed variants store. DB access only.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

const variantColumns = `id, product_id, name, sku, price_cents, is_active, created_at, updated_at`

func scanVariant(row pgx.Row) (model.Variant, error) {
	var v model.Variant
	err := row.Scan(&v.ID, &v.ProductID, &v.Name, &v.SKU, &v.PriceCents, &v.IsActive, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Variant{}, apperr.NotFound("variant not found")
	}
	if err != nil {
		return model.Variant{}, apperr.Internal(err)
	}
	return v, nil
}

// pgCode extracts the Postgres error code through the apperr wrapper.
func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func (r *Postgres) Create(ctx context.Context, v model.Variant) (model.Variant, error) {
	const q = `INSERT INTO variants (product_id, name, sku, price_cents)
	           VALUES ($1, $2, $3, $4)
	           RETURNING ` + variantColumns
	created, err := scanVariant(r.pool.QueryRow(ctx, q, v.ProductID, v.Name, v.SKU, v.PriceCents))
	if err != nil {
		switch pgCode(err) {
		case "23505":
			return model.Variant{}, apperr.Conflict("sku already in use")
		case "23503":
			// product_id FK: the product doesn't exist.
			return model.Variant{}, apperr.NotFound("product not found")
		}
		return model.Variant{}, err
	}
	return created, nil
}

func (r *Postgres) ListByProduct(ctx context.Context, productID string) ([]model.Variant, error) {
	const q = `SELECT ` + variantColumns + ` FROM variants WHERE product_id = $1 ORDER BY name`
	rows, err := r.pool.Query(ctx, q, productID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Variant{}
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	return out, nil
}

func (r *Postgres) Update(ctx context.Context, v model.Variant) (model.Variant, error) {
	const q = `UPDATE variants
	           SET name = $2, price_cents = $3, is_active = $4, updated_at = now()
	           WHERE id = $1
	           RETURNING ` + variantColumns
	return scanVariant(r.pool.QueryRow(ctx, q, v.ID, v.Name, v.PriceCents, v.IsActive))
}

func (r *Postgres) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM variants WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("variant not found")
	}
	return nil
}

func (r *Postgres) GetByID(ctx context.Context, id string) (model.Variant, error) {
	const q = `SELECT ` + variantColumns + ` FROM variants WHERE id = $1`
	return scanVariant(r.pool.QueryRow(ctx, q, id))
}
