package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/orders/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the pgx-backed orders store. Order + lines insert in one
// transaction; status moves are guarded (only pending → …) so concurrent
// pay/cancel can't both win.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

const orderColumns = `id, user_id, status, subtotal_cents, currency, created_at, updated_at`
const itemColumns = `id, order_id, product_id, variant_id, name, sku, unit_price_cents, qty`

func scanOrder(row pgx.Row) (model.Order, error) {
	var o model.Order
	err := row.Scan(&o.ID, &o.UserID, &o.Status, &o.SubtotalCents, &o.Currency, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Order{}, apperr.NotFound("order not found")
	}
	if err != nil {
		return model.Order{}, apperr.Internal(err)
	}
	return o, nil
}

func scanItem(row pgx.Row) (model.Item, error) {
	var i model.Item
	err := row.Scan(&i.ID, &i.OrderID, &i.ProductID, &i.VariantID, &i.Name, &i.SKU,
		&i.UnitPriceCents, &i.Qty)
	if err != nil {
		return model.Item{}, apperr.Internal(err)
	}
	return i, nil
}

func (r *Postgres) listItems(ctx context.Context, orderIDs []string) (map[string][]model.Item, error) {
	out := map[string][]model.Item{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	const q = `SELECT ` + itemColumns + ` FROM order_items WHERE order_id = ANY($1) ORDER BY name`
	rows, err := r.pool.Query(ctx, q, orderIDs)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	for rows.Next() {
		i, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		out[i.OrderID] = append(out[i.OrderID], i)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	return out, nil
}

// Create inserts the order header + lines atomically.
func (r *Postgres) Create(ctx context.Context, o model.Order, items []model.Item) (model.Order, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.Order{}, apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	const head = `INSERT INTO orders (user_id, status, subtotal_cents, currency)
	              VALUES ($1, $2, $3, $4)
	              RETURNING ` + orderColumns
	created, err := scanOrder(tx.QueryRow(ctx, head, o.UserID, o.Status, o.SubtotalCents, o.Currency))
	if err != nil {
		return model.Order{}, err
	}
	const line = `INSERT INTO order_items (order_id, product_id, variant_id, name, sku, unit_price_cents, qty)
	              VALUES ($1, $2, $3, $4, $5, $6, $7)`
	for _, i := range items {
		if _, err := tx.Exec(ctx, line, created.ID, i.ProductID, i.VariantID,
			i.Name, i.SKU, i.UnitPriceCents, i.Qty); err != nil {
			return model.Order{}, apperr.Internal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Order{}, apperr.Internal(err)
	}
	return created, nil
}

func (r *Postgres) GetByID(ctx context.Context, orderID string) (model.Order, error) {
	const q = `SELECT ` + orderColumns + ` FROM orders WHERE id = $1`
	return scanOrder(r.pool.QueryRow(ctx, q, orderID))
}

// ListByUser returns one filtered page of the caller's orders plus the
// total matching the same filters.
func (r *Postgres) ListByUser(ctx context.Context, userID string, f model.ListFilter) ([]model.Order, int64, error) {
	where, args := orderWhere(userID, f)

	countQ := `SELECT COUNT(*) FROM orders WHERE ` + where
	var total int64
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, apperr.Internal(err)
	}

	listArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	q := `SELECT ` + orderColumns + ` FROM orders WHERE ` + where +
		` ORDER BY ` + orderOrder(f.Sort) +
		fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, q, listArgs...)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return out, total, nil
}

func orderWhere(userID string, f model.ListFilter) (string, []any) {
	clauses := []string{"user_id = $1"}
	args := []any{userID}
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if f.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = %s", add(f.Status)))
	}
	if f.Currency != "" {
		clauses = append(clauses, fmt.Sprintf("upper(currency) = %s", add(strings.ToUpper(f.Currency))))
	}
	if f.CreatedAfter != nil {
		clauses = append(clauses, fmt.Sprintf("created_at >= %s", add(*f.CreatedAfter)))
	}
	if f.CreatedBefore != nil {
		clauses = append(clauses, fmt.Sprintf("created_at <= %s", add(*f.CreatedBefore)))
	}
	if f.MinTotalCents != nil {
		clauses = append(clauses, fmt.Sprintf("subtotal_cents >= %s", add(*f.MinTotalCents)))
	}
	if f.MaxTotalCents != nil {
		clauses = append(clauses, fmt.Sprintf("subtotal_cents <= %s", add(*f.MaxTotalCents)))
	}
	return strings.Join(clauses, " AND "), args
}

func orderOrder(sortKey string) string {
	switch sortKey {
	case "oldest":
		return "created_at ASC, id ASC"
	case "total_asc":
		return "subtotal_cents ASC, created_at DESC, id ASC"
	case "total_desc":
		return "subtotal_cents DESC, created_at DESC, id ASC"
	default: // newest
		return "created_at DESC, id ASC"
	}
}

// transition moves a pending order to status, returning the order. Any
// non-pending order (or unknown id) is NOT_FOUND-or-CONFLICT: unknown id
// reads as NotFound, wrong status as Conflict.
func (r *Postgres) transition(ctx context.Context, orderID, status string) (model.Order, error) {
	const q = `UPDATE orders SET status = $2, updated_at = now()
	           WHERE id = $1 AND status = 'pending'
	           RETURNING ` + orderColumns
	o, err := scanOrder(r.pool.QueryRow(ctx, q, orderID, status))
	if err == nil {
		return o, nil
	}
	if apperr.CodeOf(err) != apperr.CodeNotFound {
		return model.Order{}, err
	}
	// Guarded update hit nothing: unknown id, or already terminal.
	if _, getErr := r.GetByID(ctx, orderID); getErr != nil {
		return model.Order{}, getErr
	}
	return model.Order{}, apperr.Conflict("order is not pending")
}

func (r *Postgres) MarkPaid(ctx context.Context, orderID string) (model.Order, error) {
	return r.transition(ctx, orderID, "paid")
}

func (r *Postgres) Cancel(ctx context.Context, orderID string) (model.Order, error) {
	return r.transition(ctx, orderID, "cancelled")
}

func (r *Postgres) GetItems(ctx context.Context, orderID string) ([]model.Item, error) {
	items, err := r.listItems(ctx, []string{orderID})
	if err != nil {
		return nil, err
	}
	return items[orderID], nil
}
