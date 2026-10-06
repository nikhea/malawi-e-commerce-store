package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/wishlist/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the pgx-backed wishlist store. DB access only.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func (r *Postgres) Add(ctx context.Context, userID, productID string) (model.Item, error) {
	const q = `INSERT INTO wishlist_items (user_id, product_id) VALUES ($1, $2)
	           ON CONFLICT (user_id, product_id) DO NOTHING
	           RETURNING id, user_id, product_id, created_at`
	var i model.Item
	err := r.pool.QueryRow(ctx, q, userID, productID).Scan(&i.ID, &i.UserID, &i.ProductID, &i.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Conflict-no-op: link already exists, read it back.
		const get = `SELECT id, user_id, product_id, created_at
		             FROM wishlist_items WHERE user_id = $1 AND product_id = $2`
		return r.get(ctx, get, userID, productID)
	}
	if err != nil {
		return model.Item{}, apperr.Internal(err)
	}
	return i, nil
}

func (r *Postgres) get(ctx context.Context, q string, args ...any) (model.Item, error) {
	var i model.Item
	err := r.pool.QueryRow(ctx, q, args...).Scan(&i.ID, &i.UserID, &i.ProductID, &i.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Item{}, apperr.NotFound("wishlist item not found")
	}
	if err != nil {
		return model.Item{}, apperr.Internal(err)
	}
	return i, nil
}

func (r *Postgres) ListByUser(ctx context.Context, userID string) ([]model.Item, error) {
	const q = `SELECT id, user_id, product_id, created_at
	           FROM wishlist_items WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Item{}
	for rows.Next() {
		var i model.Item
		if err := rows.Scan(&i.ID, &i.UserID, &i.ProductID, &i.CreatedAt); err != nil {
			return nil, apperr.Internal(err)
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	return out, nil
}

func (r *Postgres) Remove(ctx context.Context, userID, productID string) error {
	const q = `DELETE FROM wishlist_items WHERE user_id = $1 AND product_id = $2`
	tag, err := r.pool.Exec(ctx, q, userID, productID)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("wishlist item not found")
	}
	return nil
}
