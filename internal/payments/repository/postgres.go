package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/payments/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the pgx-backed payments ledger. DB access only.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

const paymentColumns = `id, order_id, stripe_intent_id, client_secret, amount_cents, currency, order_amount_cents, status, created_at, updated_at`

func scanPayment(row pgx.Row) (model.Payment, error) {
	var p model.Payment
	err := row.Scan(&p.ID, &p.OrderID, &p.StripeIntentID, &p.ClientSecret,
		&p.AmountCents, &p.Currency, &p.OrderAmountCents, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Payment{}, apperr.NotFound("payment not found")
	}
	if err != nil {
		return model.Payment{}, apperr.Internal(err)
	}
	return p, nil
}

func (r *Postgres) Create(ctx context.Context, p model.Payment) (model.Payment, error) {
	const q = `INSERT INTO payments (order_id, stripe_intent_id, client_secret, amount_cents, currency, order_amount_cents, status)
	           VALUES ($1, $2, $3, $4, $5, $6, $7)
	           RETURNING ` + paymentColumns
	return scanPayment(r.pool.QueryRow(ctx, q, p.OrderID, p.StripeIntentID, p.ClientSecret,
		p.AmountCents, p.Currency, p.OrderAmountCents, p.Status))
}

func (r *Postgres) GetByOrder(ctx context.Context, orderID string) (model.Payment, error) {
	const q = `SELECT ` + paymentColumns + ` FROM payments WHERE order_id = $1`
	return scanPayment(r.pool.QueryRow(ctx, q, orderID))
}

func (r *Postgres) GetByIntent(ctx context.Context, intentID string) (model.Payment, error) {
	const q = `SELECT ` + paymentColumns + ` FROM payments WHERE stripe_intent_id = $1`
	return scanPayment(r.pool.QueryRow(ctx, q, intentID))
}

func (r *Postgres) MarkStatus(ctx context.Context, intentID, status string) (model.Payment, error) {
	const q = `UPDATE payments SET status = $2, updated_at = now()
	           WHERE stripe_intent_id = $1
	           RETURNING ` + paymentColumns
	return scanPayment(r.pool.QueryRow(ctx, q, intentID, status))
}
