package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/cart/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the pgx-backed cart store. DB access only. Upserts rely on
// the COALESCE unique index from migration 0006 — NULL variant lines
// merge instead of duplicating.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

const cartColumns = `id, user_id, created_at, updated_at`
const itemColumns = `id, cart_id, product_id, variant_id, name, sku, unit_price_cents, currency, qty, created_at, updated_at`

func scanCart(row pgx.Row) (model.Cart, error) {
	var c model.Cart
	err := row.Scan(&c.ID, &c.UserID, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Cart{}, apperr.NotFound("cart not found")
	}
	if err != nil {
		return model.Cart{}, apperr.Internal(err)
	}
	return c, nil
}

func scanItem(row pgx.Row) (model.Item, error) {
	var i model.Item
	err := row.Scan(&i.ID, &i.CartID, &i.ProductID, &i.VariantID, &i.Name, &i.SKU,
		&i.UnitPriceCents, &i.Currency, &i.Qty, &i.CreatedAt, &i.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Item{}, apperr.NotFound("cart item not found")
	}
	if err != nil {
		return model.Item{}, apperr.Internal(err)
	}
	return i, nil
}

// GetOrCreateCart returns the user's cart, inserting an empty one first.
func (r *Postgres) GetOrCreateCart(ctx context.Context, userID string) (model.Cart, error) {
	const q = `INSERT INTO carts (user_id) VALUES ($1)
	           ON CONFLICT (user_id) DO UPDATE SET updated_at = carts.updated_at
	           RETURNING ` + cartColumns
	return scanCart(r.pool.QueryRow(ctx, q, userID))
}

func (r *Postgres) ListItems(ctx context.Context, cartID string) ([]model.Item, error) {
	where, args := cartItemWhere(cartID, model.ListFilter{})
	q := `SELECT ` + itemColumns + ` FROM cart_items WHERE ` + where + ` ORDER BY created_at, id`
	return r.queryItems(ctx, q, args...)
}

// ListItemsPage returns one page of a cart's lines plus the total matching
// the same filters. Limit <= 0 leaves the query unbounded.
func (r *Postgres) ListItemsPage(ctx context.Context, cartID string, f model.ListFilter) ([]model.Item, int64, error) {
	where, args := cartItemWhere(cartID, f)

	countQ := `SELECT COUNT(*) FROM cart_items WHERE ` + where
	var total int64
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, apperr.Internal(err)
	}

	q := `SELECT ` + itemColumns + ` FROM cart_items WHERE ` + where +
		` ORDER BY ` + cartItemOrder(f.Sort)
	listArgs := args
	if f.Limit > 0 {
		listArgs = append(append([]any{}, args...), f.Limit, f.Offset)
		q += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	}
	items, err := r.queryItems(ctx, q, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Totals returns the whole cart's subtotal and currency, independent of
// which items page the caller requested.
func (r *Postgres) Totals(ctx context.Context, cartID string) (int64, string, error) {
	const q = `SELECT COALESCE(SUM(unit_price_cents * qty), 0)::BIGINT,
	                  COALESCE(MAX(currency), 'USD')
	           FROM cart_items WHERE cart_id = $1`
	var subtotal int64
	var currency string
	if err := r.pool.QueryRow(ctx, q, cartID).Scan(&subtotal, &currency); err != nil {
		return 0, "", apperr.Internal(err)
	}
	return subtotal, currency, nil
}

func (r *Postgres) queryItems(ctx context.Context, q string, args ...any) ([]model.Item, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Item{}
	for rows.Next() {
		i, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	return out, nil
}

func cartItemWhere(cartID string, f model.ListFilter) (string, []any) {
	clauses := []string{"cart_id = $1"}
	args := []any{cartID}
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if f.Query != "" {
		pattern := "%" + escapeLike(f.Query) + "%"
		n := add(pattern)
		clauses = append(clauses, fmt.Sprintf(
			`(lower(name) LIKE lower(%s) ESCAPE '\' OR lower(sku) LIKE lower(%s) ESCAPE '\')`, n, n))
	}
	if f.ProductID != nil {
		clauses = append(clauses, fmt.Sprintf("product_id = %s", add(*f.ProductID)))
	}
	if f.VariantID != nil {
		clauses = append(clauses, fmt.Sprintf("variant_id = %s", add(*f.VariantID)))
	}
	return strings.Join(clauses, " AND "), args
}

func cartItemOrder(sortKey string) string {
	switch sortKey {
	case "newest":
		return "created_at DESC, id ASC"
	case "name_asc":
		return "name ASC, id ASC"
	case "name_desc":
		return "name DESC, id ASC"
	case "price_asc":
		return "unit_price_cents ASC, id ASC"
	case "price_desc":
		return "unit_price_cents DESC, id ASC"
	case "qty_asc":
		return "qty ASC, id ASC"
	case "qty_desc":
		return "qty DESC, id ASC"
	default: // oldest
		return "created_at ASC, id ASC"
	}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// UpsertItem inserts the line or bumps qty on the same product/variant.
// Snapshot columns (name/sku/price) refresh to the catalog's current
// values on every add — the cart tracks the live price, checkout freezes.
func (r *Postgres) UpsertItem(ctx context.Context, i model.Item) (model.Item, error) {
	const q = `INSERT INTO cart_items (cart_id, product_id, variant_id, name, sku, unit_price_cents, currency, qty)
	           VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	           ON CONFLICT (cart_id, product_id, COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'))
	           DO UPDATE SET qty = cart_items.qty + EXCLUDED.qty,
	                         name = EXCLUDED.name, sku = EXCLUDED.sku,
	                         unit_price_cents = EXCLUDED.unit_price_cents,
	                         updated_at = now()
	           RETURNING ` + itemColumns
	return scanItem(r.pool.QueryRow(ctx, q, i.CartID, i.ProductID, i.VariantID,
		i.Name, i.SKU, i.UnitPriceCents, i.Currency, i.Qty))
}

func (r *Postgres) GetItem(ctx context.Context, cartID, itemID string) (model.Item, error) {
	const q = `SELECT ` + itemColumns + ` FROM cart_items WHERE cart_id = $1 AND id = $2`
	return scanItem(r.pool.QueryRow(ctx, q, cartID, itemID))
}

func (r *Postgres) SetQty(ctx context.Context, cartID, itemID string, qty int) (model.Item, error) {
	const q = `UPDATE cart_items SET qty = $3, updated_at = now()
	           WHERE cart_id = $1 AND id = $2
	           RETURNING ` + itemColumns
	return scanItem(r.pool.QueryRow(ctx, q, cartID, itemID, qty))
}

func (r *Postgres) DeleteItem(ctx context.Context, cartID, itemID string) error {
	const q = `DELETE FROM cart_items WHERE cart_id = $1 AND id = $2`
	tag, err := r.pool.Exec(ctx, q, cartID, itemID)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("cart item not found")
	}
	return nil
}

func (r *Postgres) Clear(ctx context.Context, cartID string) error {
	const q = `DELETE FROM cart_items WHERE cart_id = $1`
	if _, err := r.pool.Exec(ctx, q, cartID); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (r *Postgres) Touch(ctx context.Context, cartID string) error {
	const q = `UPDATE carts SET updated_at = now() WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, cartID)
	return err
}
