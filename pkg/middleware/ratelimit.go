package middleware

import (
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nikhea/malawi-e-commerce-store/pkg/apperr"
	"github.com/nikhea/malawi-e-commerce-store/pkg/response"
	"github.com/redis/go-redis/v9"
)

// fixedWindow increments the key and sets its TTL atomically, returning
// the new count. One round trip, no race between INCR and EXPIRE.
var fixedWindow = redis.NewScript(`
local count = redis.call("INCR", KEYS[1])
if count == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return count
`)

// RateLimitConfig tunes one limiter instance. Attach different instances
// per surface (strict on auth, generous on webhooks).
type RateLimitConfig struct {
	// Name namespaces keys: "rl:<name>:<ip>".
	Name string
	// Limit is max requests per Window per IP.
	Limit int
	// Window resets the counter.
	Window time.Duration
}

// RateLimit throttles by client IP (Gin resolves it honoring trusted
// proxies, which we clear). Over-limit answers 429 RATE_LIMITED with a
// Retry-After hint. Fail-open: a Redis error logs and lets the request
// through — throttling must never take the shop down.
func RateLimit(client *redis.Client, cfg RateLimitConfig) gin.HandlerFunc {
	windowMS := cfg.Window.Milliseconds()
	retryAfter := strconv.Itoa(int(cfg.Window.Seconds()))
	return func(c *gin.Context) {
		key := "rl:" + cfg.Name + ":" + c.ClientIP()
		count, err := fixedWindow.Run(c.Request.Context(), client, []string{key}, windowMS).Int()
		if err != nil {
			log.Printf("ratelimit: redis error (fail-open): %v", err)
			c.Next()
			return
		}
		if count > cfg.Limit {
			c.Header("Retry-After", retryAfter)
			response.Error(c, apperr.RateLimited("too many requests"))
			c.Abort()
			return
		}
		c.Next()
	}
}
