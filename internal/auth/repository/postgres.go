package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nikhea/malawi-e-commerce-store/internal/auth/model"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
)

// Postgres backs verification codes, reset tokens, and refresh lineage.
// DB access only: expiry checks happen here (single clock — the DB's),
// policy decisions in the service.
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

// UpsertVerification replaces any pending code (resend = new code, one
// live row per user). Returns the row for its expiry.
func (r *Postgres) UpsertVerification(ctx context.Context, userID, codeHash string, expiresAt time.Time) error {
	const q = `INSERT INTO email_verifications (user_id, code_hash, expires_at, attempts)
	           VALUES ($1, $2, $3, 0)
	           ON CONFLICT (user_id) DO UPDATE
	           SET code_hash = EXCLUDED.code_hash, expires_at = EXCLUDED.expires_at, attempts = 0`
	_, err := r.pool.Exec(ctx, q, userID, codeHash, expiresAt)
	return err
}

// ConsumeVerification atomically checks code + expiry and deletes the row.
// Wrong code bumps attempts (abuse signal for future lockout).
func (r *Postgres) ConsumeVerification(ctx context.Context, userID, codeHash string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperr.Internal(err)
	}
	defer tx.Rollback(ctx)

	var stored string
	var expires time.Time
	err = tx.QueryRow(ctx, `SELECT code_hash, expires_at FROM email_verifications
	                        WHERE user_id = $1 FOR UPDATE`, userID).Scan(&stored, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound("verification not found")
	}
	if err != nil {
		return apperr.Internal(err)
	}
	if time.Now().After(expires) {
		return apperr.Validation("code expired")
	}
	if stored != codeHash {
		_, _ = tx.Exec(ctx, `UPDATE email_verifications SET attempts = attempts + 1 WHERE user_id = $1`, userID)
		_ = tx.Commit(ctx)
		return apperr.Unauthorized("invalid code")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM email_verifications WHERE user_id = $1`, userID); err != nil {
		return apperr.Internal(err)
	}
	return tx.Commit(ctx)
}

// CreateReset stores a reset token (one live row per user: resend
// replaces). Returns nothing sensitive.
func (r *Postgres) CreateReset(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	// Delete-then-insert (not upsert): the UNIQUE is on token_hash, and
	// we want exactly one live row per user.
	if _, err := r.pool.Exec(ctx, `DELETE FROM password_resets WHERE user_id = $1`, userID); err != nil {
		return apperr.Internal(err)
	}
	_, err := r.pool.Exec(ctx, `INSERT INTO password_resets (user_id, token_hash, expires_at)
	                            VALUES ($1, $2, $3)`, userID, tokenHash, expiresAt)
	return err
}

// ConsumeReset validates and single-uses a reset token.
func (r *Postgres) ConsumeReset(ctx context.Context, tokenHash string) (model.PasswordReset, error) {
	var pr model.PasswordReset
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, token_hash, expires_at, used_at
	                             FROM password_resets WHERE token_hash = $1`, tokenHash).
		Scan(&pr.ID, &pr.UserID, &pr.TokenHash, &pr.ExpiresAt, &pr.UsedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PasswordReset{}, apperr.NotFound("reset token not found")
	}
	if err != nil {
		return model.PasswordReset{}, apperr.Internal(err)
	}
	if pr.UsedAt != nil {
		return model.PasswordReset{}, apperr.Conflict("token already used")
	}
	if time.Now().After(pr.ExpiresAt) {
		return model.PasswordReset{}, apperr.Validation("token expired")
	}
	if _, err := r.pool.Exec(ctx, `UPDATE password_resets SET used_at = now() WHERE id = $1`, pr.ID); err != nil {
		return model.PasswordReset{}, apperr.Internal(err)
	}
	return pr, nil
}

// CreateRefresh stores a refresh token, returning its id.
func (r *Postgres) CreateRefresh(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
	                             VALUES ($1, $2, $3) RETURNING id`, userID, tokenHash, expiresAt).Scan(&id)
	if err != nil {
		return "", apperr.Internal(err)
	}
	return id, nil
}

// GetRefresh fetches a live (unrevoked, unexpired) refresh token.
func (r *Postgres) GetRefresh(ctx context.Context, tokenHash string) (model.RefreshToken, error) {
	var rt model.RefreshToken
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, token_hash, expires_at, revoked_at, replaced_by
	                             FROM refresh_tokens
	                             WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`, tokenHash).
		Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.RevokedAt, &rt.ReplacedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.RefreshToken{}, apperr.NotFound("refresh token not found")
	}
	if err != nil {
		return model.RefreshToken{}, apperr.Internal(err)
	}
	return rt, nil
}

// RotateRefresh revokes the old token and links the replacement. Returns
// the old row (for user scoping) — the caller inserts the new token.
func (r *Postgres) RotateRefresh(ctx context.Context, oldID, newID string) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now(), replaced_by = $2 WHERE id = $1`
	tag, err := r.pool.Exec(ctx, q, oldID, newID)
	if err != nil {
		return apperr.Internal(err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("refresh token not found")
	}
	return nil
}

// RevokeRefresh silences one token (logout). Unknown = success.
func (r *Postgres) RevokeRefresh(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1`, tokenHash)
	return err
}

// RevokeUserRefresh kills every live token for a user (password reset,
// reuse-detected theft). Unknown users succeed silently.
func (r *Postgres) RevokeUserRefresh(ctx context.Context, userID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now()
	                            WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// RefreshUsedElsewhere detects rotation reuse: the hash exists but is
// revoked AND was replaced — someone replayed a consumed token.
func (r *Postgres) RefreshUsedElsewhere(ctx context.Context, tokenHash string) (model.RefreshToken, bool, error) {
	var rt model.RefreshToken
	err := r.pool.QueryRow(ctx, `SELECT id, user_id, token_hash, expires_at, revoked_at, replaced_by
	                             FROM refresh_tokens WHERE token_hash = $1`, tokenHash).
		Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.RevokedAt, &rt.ReplacedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.RefreshToken{}, false, nil
	}
	if err != nil {
		return model.RefreshToken{}, false, apperr.Internal(err)
	}
	if rt.RevokedAt != nil && rt.ReplacedBy != nil {
		return rt, true, nil
	}
	return rt, false, nil
}
