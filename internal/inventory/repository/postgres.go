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

func (r *Postgres) UpsertStock(ctx context.Context, productID string, variantID *string, qty int) (model.Stock, error) {
	const q = `INSERT INTO inventory (product_id, variant_id, on_hand_qty)
	           VALUES ($1, $2, $3)
	           ON CONFLICT (product_id, COALESCE(variant_id, '00000000-0000-0000-0000-000000000000'))
	           DO UPDATE SET on_hand_qty = EXCLUDED.on_hand_qty, updated_at = now()
	           RETURNING id, product_id, variant_id, on_hand_qty, reserved_qty, created_at, updated_at`
	var s model.Stock
	err := r.pool.QueryRow(ctx, q, productID, variantID, qty).Scan(
		&s.ID, &s.ProductID, &s.VariantID, &s.OnHand, &s.Reserved, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if pgCode(err) == "23503" {
			return model.Stock{}, apperr.NotFound("product not found")
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Stock{}, apperr.Internal(errors.New("upsert returned nothing"))
		}
		return model.Stock{}, apperr.Internal(err)
	}
	return s, nil
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
