package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

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

// ListByUser returns one filtered page of the caller's hearts plus the
// total matching the same filters. Products are joined only for the
// filter/sort columns; the service attaches the catalog view.
func (r *Postgres) ListByUser(ctx context.Context, userID string, f model.ListFilter) ([]model.Item, int64, error) {
	where, args := wishlistWhere(userID, f)
	const from = ` FROM wishlist_items w JOIN products p ON p.id = w.product_id`

	countQ := `SELECT COUNT(*)` + from + ` WHERE ` + where
	var total int64
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, apperr.Internal(err)
	}

	listArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	q := `SELECT w.id, w.user_id, w.product_id, w.created_at` + from +
		` WHERE ` + where + ` ORDER BY ` + wishlistOrder(f.Sort) +
		fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, q, listArgs...)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Item{}
	for rows.Next() {
		var i model.Item
		if err := rows.Scan(&i.ID, &i.UserID, &i.ProductID, &i.CreatedAt); err != nil {
			return nil, 0, apperr.Internal(err)
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return out, total, nil
}

func wishlistWhere(userID string, f model.ListFilter) (string, []any) {
	clauses := []string{"w.user_id = $1"}
	args := []any{userID}
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if f.Query != "" {
		pattern := "%" + escapeLike(f.Query) + "%"
		n := add(pattern)
		clauses = append(clauses, fmt.Sprintf(
			`(lower(p.name) LIKE lower(%s) ESCAPE '\' OR lower(p.slug) LIKE lower(%s) ESCAPE '\')`, n, n))
	}
	if f.CategoryID != nil {
		clauses = append(clauses, fmt.Sprintf("p.category_id = %s", add(*f.CategoryID)))
	}
	return strings.Join(clauses, " AND "), args
}

func wishlistOrder(sortKey string) string {
	switch sortKey {
	case "oldest":
		return "w.created_at ASC, w.id ASC"
	case "name_asc":
		return "p.name ASC, w.id ASC"
	case "name_desc":
		return "p.name DESC, w.id ASC"
	case "price_asc":
		return "p.price_cents ASC, w.id ASC"
	case "price_desc":
		return "p.price_cents DESC, w.id ASC"
	default: // newest
		return "w.created_at DESC, w.id ASC"
	}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
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
