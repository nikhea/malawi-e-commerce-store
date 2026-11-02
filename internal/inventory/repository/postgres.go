package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/inventory/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the pgx-backed inventory store. Multi-line mutations run
// in one transaction with row locks — overselling is a race, and races
// are closed here, not in the service.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// rowQuerier is satisfied by both *pgxpool.Pool and pgx.Tx, so the upsert
// body is shared by the single-line and transactional bulk paths.
type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func upsertStock(ctx context.Context, q rowQuerier, productID string, variantID *string, qty int) (model.Stock, error) {
	const sql = `INSERT INTO inventory (product_id, variant_id, on_hand_qty)
	             VALUES ($1, $2, $3)
	             ON CONFLICT (product_id, COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'))
	             DO UPDATE SET on_hand_qty = EXCLUDED.on_hand_qty, updated_at = now()
	             RETURNING id, product_id, variant_id, on_hand_qty, reserved_qty, created_at, updated_at`
	var s model.Stock
	err := q.QueryRow(ctx, sql, productID, variantID, qty).Scan(
		&s.ID, &s.ProductID, &s.VariantID, &s.OnHand, &s.Reserved, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if pgCode(err) == "23503" {
			if variantID != nil {
				return model.Stock{}, apperr.NotFound("variant not found")
			}
			return model.Stock{}, apperr.NotFound("product not found")
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Stock{}, apperr.Internal(errors.New("upsert returned nothing"))
		}
		return model.Stock{}, apperr.Internal(err)
	}
	return s, nil
}

func (r *Postgres) UpsertStock(ctx context.Context, productID string, variantID *string, qty int) (model.Stock, error) {
	return upsertStock(ctx, r.pool, productID, variantID, qty)
}

// BulkUpsertStock writes many stock rows. Non-atomic writes run one by one
// and report per-item errors; atomic writes share one transaction and roll
// back everything on the first failure. The returned error slice is
// index-aligned with items.
func (r *Postgres) BulkUpsertStock(ctx context.Context, items []model.StockInput, atomic bool) ([]model.Stock, []error, error) {
	errs := make([]error, len(items))
	if !atomic {
		out := make([]model.Stock, len(items))
		for i, item := range items {
			out[i], errs[i] = r.UpsertStock(ctx, item.ProductID, item.VariantID, item.Qty)
		}
		return out, errs, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, apperr.Internal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	out := make([]model.Stock, len(items))
	for i, item := range items {
		stock, err := upsertStock(ctx, tx, item.ProductID, item.VariantID, item.Qty)
		if err != nil {
			errs[i] = err
			return nil, errs, nil // deferred rollback undoes earlier lines
		}
		out[i] = stock
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, nil, apperr.Internal(err)
	}
	return out, errs, nil
}

// ListStockByVariantIDs returns availability for known variants. A known
// variant with no inventory row reads as zero stock; unknown ids are simply
// absent from the result (the service turns that into notFound).
func (r *Postgres) ListStockByVariantIDs(ctx context.Context, variantIDs []string) ([]model.Stock, error) {
	if len(variantIDs) == 0 {
		return []model.Stock{}, nil
	}
	const q = `SELECT v.product_id, v.id, COALESCE(i.on_hand_qty, 0), COALESCE(i.reserved_qty, 0)
	           FROM variants v
	           LEFT JOIN inventory i ON i.product_id = v.product_id AND i.variant_id = v.id
	           WHERE v.id = ANY($1)`
	rows, err := r.pool.Query(ctx, q, variantIDs)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Stock{}
	for rows.Next() {
		var s model.Stock
		var variantID string
		if err := rows.Scan(&s.ProductID, &variantID, &s.OnHand, &s.Reserved); err != nil {
			return nil, apperr.Internal(err)
		}
		s.VariantID = &variantID
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	return out, nil
}

func (r *Postgres) GetStock(ctx context.Context, productID string, variantID *string) (model.Stock, error) {
	const q = `SELECT id, product_id, variant_id, on_hand_qty, reserved_qty, created_at, updated_at
	           FROM inventory WHERE product_id = $1
	           AND COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE($2, '00000000-0000-0000-0000-000000000000'::uuid)`
	var s model.Stock
	err := r.pool.QueryRow(ctx, q, productID, variantID).Scan(
		&s.ID, &s.ProductID, &s.VariantID, &s.OnHand, &s.Reserved, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// Untracked SKU: zero stock, not an error.
		return model.Stock{ProductID: productID, VariantID: variantID}, nil
	}
	if err != nil {
		return model.Stock{}, apperr.Internal(err)
	}
	return s, nil
}

// ListStockByProduct returns all stock rows for a product (base row plus
// per-variant rows). Unknown product = empty list, not an error.
func (r *Postgres) ListStockByProduct(ctx context.Context, productID string) ([]model.Stock, error) {
	const q = `SELECT id, product_id, variant_id, on_hand_qty, reserved_qty, created_at, updated_at
	           FROM inventory WHERE product_id = $1
	           ORDER BY variant_id NULLS FIRST`
	rows, err := r.pool.Query(ctx, q, productID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Stock{}
	for rows.Next() {
		var s model.Stock
		if err := rows.Scan(&s.ID, &s.ProductID, &s.VariantID, &s.OnHand, &s.Reserved, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, apperr.Internal(err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	return out, nil
}

// Reserve locks every line's row, checks availability, bumps reserved,
// and writes the ledger — atomically. Any short line aborts all of them.
func (r *Postgres) Reserve(ctx context.Context, orderRef string, lines []model.Reservation, expiresAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	for _, l := range lines {
		const lock = `SELECT on_hand_qty, reserved_qty FROM inventory
		              WHERE product_id = $1
		              AND COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE($2, '00000000-0000-0000-0000-000000000000'::uuid)
		              FOR UPDATE`
		var onHand, reserved int
		err := tx.QueryRow(ctx, lock, l.ProductID, l.VariantID).Scan(&onHand, &reserved)
		if errors.Is(err, pgx.ErrNoRows) {
			return apperr.Conflict("insufficient stock")
		}
		if err != nil {
			return apperr.Internal(err)
		}
		if onHand-reserved < l.Qty {
			return apperr.Conflict("insufficient stock")
		}

		const bump = `UPDATE inventory SET reserved_qty = reserved_qty + $3, updated_at = now()
		              WHERE product_id = $1
		              AND COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE($2, '00000000-0000-0000-0000-000000000000'::uuid)`
		if _, err := tx.Exec(ctx, bump, l.ProductID, l.VariantID, l.Qty); err != nil {
			return apperr.Internal(err)
		}
		const ledger = `INSERT INTO reservations (product_id, variant_id, qty, order_ref, expires_at)
		                VALUES ($1, $2, $3, $4, $5)`
		if _, err := tx.Exec(ctx, ledger, l.ProductID, l.VariantID, l.Qty, orderRef, expiresAt); err != nil {
			return apperr.Internal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

// settle moves an order's ACTIVE holds to a terminal status, adjusting
// counters accordingly. confirm=true sells (on_hand down), false releases.
// Idempotent: no active holds is a success (webhook retries demand it).
func (r *Postgres) settle(ctx context.Context, orderRef string, confirm bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	const list = `SELECT id, product_id, variant_id, qty FROM reservations
	              WHERE order_ref = $1 AND status = 'active' FOR UPDATE`
	rows, err := tx.Query(ctx, list, orderRef)
	if err != nil {
		return apperr.Internal(err)
	}
	type hold struct {
		id        string
		productID string
		variantID *string
		qty       int
	}
	holds := []hold{}
	for rows.Next() {
		var h hold
		if err := rows.Scan(&h.id, &h.productID, &h.variantID, &h.qty); err != nil {
			rows.Close()
			return apperr.Internal(err)
		}
		holds = append(holds, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return apperr.Internal(err)
	}

	status := "released"
	for _, h := range holds {
		adjust := `UPDATE inventory SET reserved_qty = reserved_qty - $3, updated_at = now()
		           WHERE product_id = $1
		           AND COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE($2, '00000000-0000-0000-0000-000000000000'::uuid)`
		if confirm {
			status = "confirmed"
			adjust = `UPDATE inventory SET reserved_qty = reserved_qty - $3, on_hand_qty = on_hand_qty - $3, updated_at = now()
			          WHERE product_id = $1
			          AND COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'::uuid) = COALESCE($2, '00000000-0000-0000-0000-000000000000'::uuid)`
		}
		if _, err := tx.Exec(ctx, adjust, h.productID, h.variantID, h.qty); err != nil {
			return apperr.Internal(err)
		}
		if _, err := tx.Exec(ctx, `UPDATE reservations SET status = $2 WHERE id = $1`, h.id, status); err != nil {
			return apperr.Internal(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return apperr.Internal(err)
	}
	return nil
}

func (r *Postgres) ReleaseByOrder(ctx context.Context, orderRef string) error {
	return r.settle(ctx, orderRef, false)
}

func (r *Postgres) ConfirmByOrder(ctx context.Context, orderRef string) error {
	return r.settle(ctx, orderRef, true)
}

// ReleaseExpired frees every hold past its TTL (abandoned checkouts)
// and returns the affected order refs for order cancellation. Each ref
// settles in its own transaction — one poisoned ref can't block the rest.
func (r *Postgres) ReleaseExpired(ctx context.Context) ([]string, error) {
	const q = `SELECT DISTINCT order_ref FROM reservations
	           WHERE status = 'active' AND expires_at < now()`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	refs := []string{}
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			rows.Close()
			return nil, apperr.Internal(err)
		}
		refs = append(refs, ref)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	for _, ref := range refs {
		if err := r.settle(ctx, ref, false); err != nil {
			return refs, err
		}
	}
	return refs, nil
}
