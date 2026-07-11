package httputil

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// RateAllower is the minimal slice of cache.RateLimiter this package needs,
// declared locally so shared/ does not import infra/.
type RateAllower interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

// EnforceUserRateLimit checks the per-user limit for action. It returns true
// when the request may proceed. On breach it writes 429 and returns false.
// A limiter error fails open (allow) so a Redis hiccup never blocks legit users.
func EnforceUserRateLimit(w http.ResponseWriter, r *http.Request, limiter RateAllower, action string, limit int, window time.Duration) bool {
	sess := SessionFromContext(r.Context())
	if sess == nil {
		return true // unauthenticated routes are not rate-limited here
	}
	key := action + ":" + sess.UserID
	ok, err := limiter.Allow(r.Context(), key, limit, window)
	if err != nil {
		slog.Error("rate limit check failed, allowing", "action", action, "error", err)
		return true
	}
	if !ok {
		Error(w, http.StatusTooManyRequests, "too many uploads, try again in a minute")
		return false
	}
	return true
}
