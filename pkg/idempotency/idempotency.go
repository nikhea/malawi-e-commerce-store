// Package idempotency stores bulk-write responses in Redis so retrying a
// request with the same Idempotency-Key and body replays the first result
// instead of running the writes twice. Keys are scoped per authenticated
// user and expire after 24 hours.
package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/middleware"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
	"github.com/redis/go-redis/v9"
)

const (
	keyPrefix  = "idem:v1:"
	defaultTTL = 24 * time.Hour
	maxKeyLen  = 200
)

// Store is the Redis-backed idempotency cache. A nil *Store is valid and
// disables the middleware (Redis down / tests without Redis).
type Store struct {
	client *redis.Client
	ttl    time.Duration
}

// New returns nil when no Redis client is available, matching the
// rate-limiter's fail-open posture.
func New(client *redis.Client) *Store {
	if client == nil {
		return nil
	}
	return &Store{client: client, ttl: defaultTTL}
}

type record struct {
	Hash   string `json:"hash"`
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// Middleware replays cached responses and rejects key reuse with a
// different request. It caches only 2xx responses: a 400/409 means the
// caller should fix the request, not replay it.
func (s *Store) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s == nil || s.client == nil {
			c.Next()
			return
		}
		key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
		if key == "" {
			c.Next()
			return
		}
		if len(key) > maxKeyLen {
			response.Error(c, apperr.Validation("idempotency key is too long"))
			c.Abort()
			return
		}

		body, err := readBody(c)
		if err != nil {
			response.Error(c, apperr.Internal(err))
			c.Abort()
			return
		}
		hash := hashRequest(c, body)
		redisKey := keyPrefix + scopeOf(c) + ":" + key
		ctx := c.Request.Context()

		if cached, ok := s.load(ctx, redisKey, hash); ok {
			c.Data(cached.Status, "application/json; charset=utf-8", []byte(cached.Body))
			c.Abort()
			return
		} else if cached != nil {
			// Same key, different request: never execute the second shape.
			response.Error(c, apperr.IdempotencyConflict("idempotency key reused with a different request"))
			c.Abort()
			return
		}

		capture := &captureWriter{ResponseWriter: c.Writer}
		c.Writer = capture
		c.Next()

		status := capture.Status()
		if status < http.StatusOK || status >= http.StatusMultipleChoices {
			return
		}
		rec := record{Hash: hash, Status: status, Body: capture.body.String()}
		raw, err := json.Marshal(rec)
		if err != nil {
			log.Printf("idempotency: encode: %v", err)
			return
		}
		if err := s.client.Set(ctx, redisKey, raw, s.ttl).Err(); err != nil {
			// Fail-open: the write already happened; losing the cache only
			// risks a duplicate on retry, which the DB guards handle.
			log.Printf("idempotency: redis set (fail-open): %v", err)
		}
	}
}

// load returns (record, true) on a matching hash, (record, false) on a
// hash mismatch, and (nil, false) on a miss or a Redis error.
func (s *Store) load(ctx context.Context, key, hash string) (*record, bool) {
	raw, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false
	}
	if err != nil {
		log.Printf("idempotency: redis get (fail-open): %v", err)
		return nil, false
	}
	var rec record
	if err := json.Unmarshal(raw, &rec); err != nil {
		log.Printf("idempotency: decode cached response (ignored): %v", err)
		return nil, false
	}
	if rec.Hash != hash {
		return &rec, false
	}
	return &rec, true
}

func scopeOf(c *gin.Context) string {
	if id := middleware.UserIDOf(c); id != "" {
		return id
	}
	return "anon"
}

func readBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// hashRequest binds the key to one method + URI + body. A key reused on a
// different endpoint is a conflict, not a replay of the wrong response.
func hashRequest(c *gin.Context, body []byte) string {
	sum := sha256.New()
	_, _ = sum.Write([]byte(c.Request.Method))
	_, _ = sum.Write([]byte(" "))
	_, _ = sum.Write([]byte(c.Request.URL.RequestURI()))
	_, _ = sum.Write([]byte("\n"))
	_, _ = sum.Write(body)
	return hex.EncodeToString(sum.Sum(nil))
}

// captureWriter tees the response body so Middleware can cache it after
// the handler runs. Everything else delegates to Gin's writer.
type captureWriter struct {
	gin.ResponseWriter
	body bytes.Buffer
}

func (w *captureWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}
