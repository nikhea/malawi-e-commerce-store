package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/reviews/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres is the pgx-backed reviews store. DB access only.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

const reviewColumns = `id, user_id, author, product_id, rating, title, body, created_at, updated_at`

func scanReview(row pgx.Row) (model.Review, error) {
	var r model.Review
	err := row.Scan(&r.ID, &r.UserID, &r.Author, &r.ProductID, &r.Rating, &r.Title,
		&r.Body, &r.CreatedAt, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Review{}, apperr.NotFound("review not found")
	}
	if err != nil {
		return model.Review{}, apperr.Internal(err)
	}
	return r, nil
}

func (r *Postgres) Create(ctx context.Context, rev model.Review) (model.Review, error) {
	const q = `INSERT INTO reviews (user_id, author, product_id, rating, title, body)
	           VALUES ($1, $2, $3, $4, $5, $6)
	           RETURNING ` + reviewColumns
	created, err := scanReview(r.pool.QueryRow(ctx, q, rev.UserID, rev.Author,
		rev.ProductID, rev.Rating, rev.Title, rev.Body))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return model.Review{}, apperr.Conflict("review already exists")
			case "23503":
				return model.Review{}, apperr.NotFound("product not found")
			}
		}
		return model.Review{}, err
	}
	return created, nil
}

func (r *Postgres) GetByID(ctx context.Context, id string) (model.Review, error) {
	const q = `SELECT ` + reviewColumns + ` FROM reviews WHERE id = $1`
	return scanReview(r.pool.QueryRow(ctx, q, id))
}

func (r *Postgres) ListByProduct(ctx context.Context, productID string) ([]model.Review, error) {
	const q = `SELECT ` + reviewColumns + ` FROM reviews WHERE product_id = $1 ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, q, productID)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Review{}
	for rows.Next() {
		rev, err := scanReview(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rev)
	}
	if err := rows.Err(); err != nil {
		return nil, apperr.Internal(err)
	}
	return out, nil
}

// Stats aggregates in SQL: one round trip, exact average.
func (r *Postgres) Stats(ctx context.Context, productID string) (avg float64, count int, err error) {
	const q = `SELECT COALESCE(AVG(rating), 0), COUNT(*) FROM reviews WHERE product_id = $1`
	if err := r.pool.QueryRow(ctx, q, productID).Scan(&avg, &count); err != nil {
		return 0, 0, apperr.Internal(err)
	}
	return avg, count, nil
}

func (r *Postgres) Update(ctx context.Context, rev model.Review) (model.Review, error) {
	const q = `UPDATE reviews SET rating = $2, title = $3, body = $4, updated_at = now()
	           WHERE id = $1
	           RETURNING ` + reviewColumns
	return scanReview(r.pool.QueryRow(ctx, q, rev.ID, rev.Rating, rev.Title, rev.Body))
}

func (r *Postgres) Delete(ctx context.Context, id string) error {
	const q = `DELETE FROM reviews WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("review not found")
	}
	return nil
}
