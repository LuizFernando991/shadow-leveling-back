package httputil

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LuizFernando991/gym-api/internal/shared/entities"
)

type stubLimiter struct {
	allow bool
	err   error
}

func (s stubLimiter) Allow(context.Context, string, int, time.Duration) (bool, error) {
	return s.allow, s.err
}

func reqWithUser(userID string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/x", nil)
	if userID != "" {
		ctx := ContextWithSession(r.Context(), &entities.Session{UserID: userID})
		r = r.WithContext(ctx)
	}
	return r
}

func TestEnforceRateLimit(t *testing.T) {
	cases := []struct {
		name     string
		limiter  stubLimiter
		wantPass bool
		wantCode int
	}{
		{"within limit", stubLimiter{allow: true}, true, 200},
		{"over limit", stubLimiter{allow: false}, false, http.StatusTooManyRequests},
		{"limiter error fails open", stubLimiter{err: context.DeadlineExceeded}, true, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/x", nil)
			got := EnforceRateLimit(w, r, c.limiter, "email-code:a@b.com", 1, time.Minute)
			if got != c.wantPass {
				t.Errorf("pass = %v, want %v", got, c.wantPass)
			}
			if !c.wantPass && w.Code != c.wantCode {
				t.Errorf("code = %d, want %d", w.Code, c.wantCode)
			}
		})
	}
}

func TestEnforceUserRateLimit(t *testing.T) {
	cases := []struct {
		name     string
		limiter  stubLimiter
		userID   string
		wantPass bool
		wantCode int
	}{
		{"within limit", stubLimiter{allow: true}, "u1", true, 200},
		{"over limit", stubLimiter{allow: false}, "u1", false, http.StatusTooManyRequests},
		{"limiter error fails open", stubLimiter{err: context.DeadlineExceeded}, "u1", true, 200},
		{"no session skips", stubLimiter{allow: false}, "", true, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			got := EnforceUserRateLimit(w, reqWithUser(c.userID), c.limiter, "upload", 10, time.Minute)
			if got != c.wantPass {
				t.Errorf("pass = %v, want %v", got, c.wantPass)
			}
			if !c.wantPass && w.Code != c.wantCode {
				t.Errorf("code = %d, want %d", w.Code, c.wantCode)
			}
		})
	}
}
