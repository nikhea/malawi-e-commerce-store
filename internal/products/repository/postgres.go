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
	"github.com/nikhea/malawi-e-commerce-store/internal/products/utils"
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

// List returns one page plus the total number of rows matching the same
// filters. A non-empty query runs full-text search first; when that finds
// nothing, the existing substring search runs so partial terms still work.
func (r *Postgres) List(ctx context.Context, f model.ListFilter) ([]model.Product, int64, error) {
	tsQuery := utils.TSQuery(f.Query)
	if tsQuery == "" {
		return r.list(ctx, f, "")
	}
	products, total, err := r.list(ctx, f, tsQuery)
	if err != nil || total > 0 {
		return products, total, err
	}
	// No word-prefix matches: keep ILIKE as a safety net for infix and
	// punctuation-heavy queries.
	return r.list(ctx, f, "")
}

// list runs the count + page pair for one search mode. tsQuery == "" means
// substring mode; otherwise the catalog is filtered by a full-text match.
func (r *Postgres) list(ctx context.Context, f model.ListFilter, tsQuery string) ([]model.Product, int64, error) {
	where, args, tsParam := productWhere(f, tsQuery)

	countQ := `SELECT COUNT(*) ` + productFrom + ` WHERE ` + where
	var total int64
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, apperr.Internal(err)
	}

	listArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	q := `SELECT ` + productColumns + ` ` + productFrom + ` WHERE ` + where +
		` ORDER BY ` + productOrder(f.Sort, tsParam) +
		fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, q, listArgs...)
	if err != nil {
		return nil, 0, apperr.Internal(err)
	}
	defer rows.Close()
	out := []model.Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperr.Internal(err)
	}
	return out, total, nil
}

// productWhere translates the validated filter into a parameterized
// predicate. Values never reach the SQL string; only placeholders do.
// tsQuery non-empty selects full-text mode; empty selects substring mode.
// The returned tsParam is the tsquery placeholder reused by ORDER BY
// (empty in substring mode).
func productWhere(f model.ListFilter, tsQuery string) (string, []any, string) {
	clauses := []string{"1 = 1"}
	args := []any{}
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	tsParam := ""

	if f.ActiveOnly {
		clauses = append(clauses, "p.is_active")
	}
	if f.Query != "" {
		if tsQuery != "" {
			tsParam = add(tsQuery)
			clauses = append(clauses, fmt.Sprintf(
				`p.search_vector @@ to_tsquery('english', %s)`, tsParam))
		} else {
			pattern := "%" + escapeLike(f.Query) + "%"
			n := add(pattern)
			// lower() mirrors the trigram indexes in 0014; ILIKE alone
			// would not match an index on lower(column).
			clauses = append(clauses, fmt.Sprintf(
				`(lower(p.name) LIKE lower(%s) ESCAPE '\' OR lower(p.description) LIKE lower(%s) ESCAPE '\' OR lower(p.slug) LIKE lower(%s) ESCAPE '\')`,
				n, n, n))
		}
	}
	if f.CategoryID != nil {
		clauses = append(clauses, fmt.Sprintf("p.category_id = %s", add(*f.CategoryID)))
	}
	if f.CategorySlug != nil {
		clauses = append(clauses, fmt.Sprintf("lower(c.slug) = lower(%s)", add(*f.CategorySlug)))
	}
	if f.MinPriceCents != nil {
		clauses = append(clauses, fmt.Sprintf("p.price_cents >= %s", add(*f.MinPriceCents)))
	}
	if f.MaxPriceCents != nil {
		clauses = append(clauses, fmt.Sprintf("p.price_cents <= %s", add(*f.MaxPriceCents)))
	}
	if f.InStock != nil {
		const stocked = `EXISTS (SELECT 1 FROM inventory i
			WHERE i.product_id = p.id AND i.on_hand_qty - i.reserved_qty > 0)`
		if *f.InStock {
			clauses = append(clauses, stocked)
		} else {
			clauses = append(clauses, "NOT "+stocked)
		}
	}
	if f.Size != "" {
		clauses = append(clauses, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM variants v WHERE v.product_id = p.id
			 AND v.is_active AND lower(v.name) = lower(%s))`, add(f.Size)))
	}
	if f.MinRating != nil {
		clauses = append(clauses, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM reviews r WHERE r.product_id = p.id
			 GROUP BY r.product_id HAVING AVG(r.rating) >= %s)`, add(*f.MinRating)))
	}
	if f.CreatedAfter != nil {
		clauses = append(clauses, fmt.Sprintf("p.created_at >= %s", add(*f.CreatedAfter)))
	}
	if f.CreatedBefore != nil {
		clauses = append(clauses, fmt.Sprintf("p.created_at <= %s", add(*f.CreatedBefore)))
	}
	return strings.Join(clauses, " AND "), args, tsParam
}

// productOrder is a closed whitelist: never interpolate a client sort
// string. Every order ends with p.id so equal keys stay deterministic
// across pages. tsParam is empty in substring mode, where relevance falls
// back to name order.
func productOrder(sortKey, tsParam string) string {
	switch sortKey {
	case "name_desc":
		return "p.name DESC, p.id ASC"
	case "price_asc":
		return "p.price_cents ASC, p.name ASC, p.id ASC"
	case "price_desc":
		return "p.price_cents DESC, p.name ASC, p.id ASC"
	case "newest":
		return "p.created_at DESC, p.id ASC"
	case "oldest":
		return "p.created_at ASC, p.id ASC"
	case "rating_desc":
		return "(SELECT AVG(r.rating) FROM reviews r WHERE r.product_id = p.id) DESC NULLS LAST, p.name ASC, p.id ASC"
	case "rating_asc":
		return "(SELECT AVG(r.rating) FROM reviews r WHERE r.product_id = p.id) ASC NULLS LAST, p.name ASC, p.id ASC"
	case "relevance":
		if tsParam == "" {
			return "p.name ASC, p.id ASC"
		}
		return fmt.Sprintf(
			"ts_rank_cd(p.search_vector, to_tsquery('english', %s)) DESC, p.name ASC, p.id ASC",
			tsParam)
	default: // name_asc
		return "p.name ASC, p.id ASC"
	}
}

// escapeLike makes user input literal inside an ILIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
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
