package auth_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/LuizFernando991/gym-api/internal/testutil"
)

var (
	srv *httptest.Server
	db  *sql.DB
)

func TestMain(m *testing.M) {
	var err error
	var teardown func()

	srv, db, teardown, err = testutil.Setup()
	if err != nil {
		fmt.Fprintf(os.Stderr, "test setup failed: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	teardown()
	os.Exit(code)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func truncate(t *testing.T) {
	t.Helper()
	if err := testutil.Truncate(db); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func request(t *testing.T, method, path string, body any, token string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req, err := http.NewRequest(method, srv.URL+path, &buf)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	return resp
}

func decodeBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
}

func assertStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Errorf("status: got %d, want %d", resp.StatusCode, want)
	}
}

// mustGetCode reads the latest verification code for an email from the DB,
// bypassing the email provider entirely. The passwordless flow uses the
// "login" verification type for both sign-up and sign-in.
func mustGetCode(t *testing.T, emailAddr string) string {
	t.Helper()
	code, err := testutil.LatestVerificationCode(db, emailAddr, "login")
	if err != nil {
		t.Fatalf("get verification code: %v", err)
	}
	return code
}

// mustAuth runs the full passwordless flow (request code → verify) and returns
// the session token. Creates the account on first use, logs in thereafter.
func mustAuth(t *testing.T, emailAddr string) string {
	t.Helper()

	resp := request(t, http.MethodPost, "/auth/email/request", map[string]string{
		"email": emailAddr,
	}, "")
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("mustAuth: POST /auth/email/request got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	code := mustGetCode(t, emailAddr)

	resp2 := request(t, http.MethodPost, "/auth/email/verify", map[string]string{
		"email": emailAddr, "code": code,
	}, "")
	if resp2.StatusCode != http.StatusOK {
		resp2.Body.Close()
		t.Fatalf("mustAuth: POST /auth/email/verify got %d, want 200", resp2.StatusCode)
	}
	var body struct {
		Token string `json:"token"`
	}
	decodeBody(t, resp2, &body)
	return body.Token
}

// ── POST /auth/email/request ──────────────────────────────────────────────────

func TestRequestEmailCode(t *testing.T) {
	t.Run("sends code and returns 200 for a new email", func(t *testing.T) {
		truncate(t)
		resp := request(t, http.MethodPost, "/auth/email/request", map[string]string{
			"email": "new@example.com",
		}, "")
		assertStatus(t, resp, http.StatusOK)

		var body struct {
			Message string `json:"message"`
		}
		decodeBody(t, resp, &body)
		if body.Message == "" {
			t.Error("expected non-empty message")
		}
		if _, err := testutil.LatestVerificationCode(db, "new@example.com", "login"); err != nil {
			t.Errorf("expected a code row to be created: %v", err)
		}
	})

	t.Run("works for an existing user", func(t *testing.T) {
		truncate(t)
		mustAuth(t, "existing@example.com")

		resp := request(t, http.MethodPost, "/auth/email/request", map[string]string{
			"email": "existing@example.com",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusOK)
	})

	t.Run("resending supersedes the previous code", func(t *testing.T) {
		truncate(t)
		resp := request(t, http.MethodPost, "/auth/email/request", map[string]string{
			"email": "resend@example.com",
		}, "")
		resp.Body.Close()
		first := mustGetCode(t, "resend@example.com")

		resp2 := request(t, http.MethodPost, "/auth/email/request", map[string]string{
			"email": "resend@example.com",
		}, "")
		resp2.Body.Close()
		second := mustGetCode(t, "resend@example.com")

		if first == second {
			t.Fatal("expected a new code on resend")
		}
		// The superseded code no longer verifies; the new one does.
		old := request(t, http.MethodPost, "/auth/email/verify", map[string]string{
			"email": "resend@example.com", "code": first,
		}, "")
		defer old.Body.Close()
		assertStatus(t, old, http.StatusUnprocessableEntity)

		fresh := request(t, http.MethodPost, "/auth/email/verify", map[string]string{
			"email": "resend@example.com", "code": second,
		}, "")
		defer fresh.Body.Close()
		assertStatus(t, fresh, http.StatusOK)
	})

	t.Run("rejects invalid email format", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/email/request", map[string]string{
			"email": "not-an-email",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects missing email", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/email/request", map[string]string{}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects empty body", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/email/request", nil, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})
}

// ── POST /auth/email/verify ───────────────────────────────────────────────────

func TestVerifyEmailCode(t *testing.T) {
	t.Run("creates account and returns token for a new email", func(t *testing.T) {
		truncate(t)
		resp := request(t, http.MethodPost, "/auth/email/request", map[string]string{
			"email": "signup@example.com",
		}, "")
		resp.Body.Close()

		code := mustGetCode(t, "signup@example.com")
		resp2 := request(t, http.MethodPost, "/auth/email/verify", map[string]string{
			"email": "signup@example.com", "code": code,
		}, "")
		assertStatus(t, resp2, http.StatusOK)

		var body struct {
			Token string `json:"token"`
		}
		decodeBody(t, resp2, &body)
		if body.Token == "" {
			t.Fatal("expected non-empty token")
		}

		// The token authenticates as the newly-created user.
		meResp := request(t, http.MethodGet, "/auth/me", nil, body.Token)
		var me struct {
			Email string `json:"email"`
		}
		decodeBody(t, meResp, &me)
		if me.Email != "signup@example.com" {
			t.Errorf("me email: got %q, want %q", me.Email, "signup@example.com")
		}
	})

	t.Run("logs in an existing user without duplicating the account", func(t *testing.T) {
		truncate(t)
		first := mustAuth(t, "return@example.com")
		second := mustAuth(t, "return@example.com")
		if first == second {
			t.Error("expected a distinct session token on second auth")
		}
		// Both tokens resolve to the same user (same email).
		for _, tok := range []string{first, second} {
			meResp := request(t, http.MethodGet, "/auth/me", nil, tok)
			var me struct {
				Email string `json:"email"`
			}
			decodeBody(t, meResp, &me)
			if me.Email != "return@example.com" {
				t.Errorf("me email: got %q, want %q", me.Email, "return@example.com")
			}
		}
	})

	t.Run("rejects wrong code", func(t *testing.T) {
		truncate(t)
		resp := request(t, http.MethodPost, "/auth/email/request", map[string]string{
			"email": "wrongcode@example.com",
		}, "")
		resp.Body.Close()

		resp2 := request(t, http.MethodPost, "/auth/email/verify", map[string]string{
			"email": "wrongcode@example.com", "code": "000000",
		}, "")
		defer resp2.Body.Close()
		assertStatus(t, resp2, http.StatusUnprocessableEntity)
	})

	t.Run("rejects expired code", func(t *testing.T) {
		truncate(t)
		if _, err := db.Exec(
			`INSERT INTO email_verifications (email, code, type, expires_at)
			 VALUES ($1, $2, 'login', $3)`,
			"expired@example.com", "654321", time.Now().Add(-time.Minute),
		); err != nil {
			t.Fatalf("insert expired code: %v", err)
		}

		resp := request(t, http.MethodPost, "/auth/email/verify", map[string]string{
			"email": "expired@example.com", "code": "654321",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusUnprocessableEntity)
	})

	t.Run("rejects code with non-digit characters", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/email/verify", map[string]string{
			"email": "user@example.com", "code": "abc123",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects code with wrong length", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/email/verify", map[string]string{
			"email": "user@example.com", "code": "123",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("code can only be used once", func(t *testing.T) {
		truncate(t)
		resp := request(t, http.MethodPost, "/auth/email/request", map[string]string{
			"email": "once@example.com",
		}, "")
		resp.Body.Close()

		code := mustGetCode(t, "once@example.com")

		resp2 := request(t, http.MethodPost, "/auth/email/verify", map[string]string{
			"email": "once@example.com", "code": code,
		}, "")
		assertStatus(t, resp2, http.StatusOK)
		resp2.Body.Close()

		resp3 := request(t, http.MethodPost, "/auth/email/verify", map[string]string{
			"email": "once@example.com", "code": code,
		}, "")
		defer resp3.Body.Close()
		assertStatus(t, resp3, http.StatusUnprocessableEntity)
	})
}

// ── GET /auth/me ──────────────────────────────────────────────────────────────

func TestMe(t *testing.T) {
	truncate(t)
	token := mustAuth(t, "me@example.com")

	t.Run("returns current user for valid token", func(t *testing.T) {
		resp := request(t, http.MethodGet, "/auth/me", nil, token)
		assertStatus(t, resp, http.StatusOK)

		var body struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			CreatedAt string `json:"created_at"`
		}
		decodeBody(t, resp, &body)
		if body.Email != "me@example.com" {
			t.Errorf("email: got %q, want %q", body.Email, "me@example.com")
		}
		if body.ID == "" {
			t.Error("expected non-empty id")
		}
	})

	t.Run("rejects request without token", func(t *testing.T) {
		resp := request(t, http.MethodGet, "/auth/me", nil, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("rejects invalid token", func(t *testing.T) {
		resp := request(t, http.MethodGet, "/auth/me", nil, "notavalidtoken")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusUnauthorized)
	})
}

// ── POST /auth/logout ─────────────────────────────────────────────────────────

func TestLogout(t *testing.T) {
	t.Run("invalidates the session token", func(t *testing.T) {
		truncate(t)
		token := mustAuth(t, "logout@example.com")

		resp := request(t, http.MethodPost, "/auth/logout", nil, token)
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusNoContent)

		resp2 := request(t, http.MethodGet, "/auth/me", nil, token)
		defer resp2.Body.Close()
		assertStatus(t, resp2, http.StatusUnauthorized)
	})

	t.Run("rejects request without token", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/logout", nil, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusUnauthorized)
	})
}

// ── GET /auth/sessions ────────────────────────────────────────────────────────

func TestListSessions(t *testing.T) {
	t.Run("returns all active sessions for the user", func(t *testing.T) {
		truncate(t)
		token := mustAuth(t, "sessions@example.com")
		mustAuth(t, "sessions@example.com")

		resp := request(t, http.MethodGet, "/auth/sessions", nil, token)
		assertStatus(t, resp, http.StatusOK)

		var sessions []struct {
			ID        string  `json:"id"`
			UserAgent string  `json:"user_agent"`
			CreatedAt string  `json:"created_at"`
			ExpiresAt *string `json:"expires_at"`
		}
		decodeBody(t, resp, &sessions)
		if len(sessions) != 2 {
			t.Fatalf("session count: got %d, want 2", len(sessions))
		}
		for i, s := range sessions {
			if s.ID == "" {
				t.Errorf("session[%d]: expected non-empty id", i)
			}
		}
	})

	t.Run("excludes sessions revoked by logout", func(t *testing.T) {
		truncate(t)
		token := mustAuth(t, "revcheck@example.com")
		tok2 := mustAuth(t, "revcheck@example.com")

		logout := request(t, http.MethodPost, "/auth/logout", nil, tok2)
		logout.Body.Close()

		resp := request(t, http.MethodGet, "/auth/sessions", nil, token)
		assertStatus(t, resp, http.StatusOK)

		var sessions []map[string]any
		decodeBody(t, resp, &sessions)
		if len(sessions) != 1 {
			t.Fatalf("session count after revoke: got %d, want 1", len(sessions))
		}
	})

	t.Run("does not expose ip_address field", func(t *testing.T) {
		truncate(t)
		token := mustAuth(t, "noip@example.com")

		resp := request(t, http.MethodGet, "/auth/sessions", nil, token)
		assertStatus(t, resp, http.StatusOK)

		var sessions []map[string]any
		decodeBody(t, resp, &sessions)
		if len(sessions) == 0 {
			t.Fatal("expected at least one session")
		}
		if _, exists := sessions[0]["ip_address"]; exists {
			t.Error("response must not contain ip_address field")
		}
	})

	t.Run("does not expose revoked_at field", func(t *testing.T) {
		truncate(t)
		token := mustAuth(t, "norevokedat@example.com")

		resp := request(t, http.MethodGet, "/auth/sessions", nil, token)
		assertStatus(t, resp, http.StatusOK)

		var sessions []map[string]any
		decodeBody(t, resp, &sessions)
		if len(sessions) == 0 {
			t.Fatal("expected at least one session")
		}
		if _, exists := sessions[0]["revoked_at"]; exists {
			t.Error("response must not contain revoked_at field")
		}
	})

	t.Run("rejects request without token", func(t *testing.T) {
		resp := request(t, http.MethodGet, "/auth/sessions", nil, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusUnauthorized)
	})
}

// ── DELETE /auth/sessions/{id} ────────────────────────────────────────────────

func TestRevokeSession(t *testing.T) {
	t.Run("revokes a session and removes it from the list", func(t *testing.T) {
		truncate(t)
		token := mustAuth(t, "revoke@example.com")
		mustAuth(t, "revoke@example.com")

		listResp := request(t, http.MethodGet, "/auth/sessions", nil, token)
		var sessions []struct {
			ID string `json:"id"`
		}
		decodeBody(t, listResp, &sessions)
		if len(sessions) != 2 {
			t.Fatalf("expected 2 sessions before revoke, got %d", len(sessions))
		}

		resp := request(t, http.MethodDelete, "/auth/sessions/"+sessions[0].ID, nil, token)
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusNoContent)

		listResp2 := request(t, http.MethodGet, "/auth/sessions", nil, token)
		var remaining []map[string]any
		decodeBody(t, listResp2, &remaining)
		if len(remaining) != 1 {
			t.Fatalf("expected 1 session after revoke, got %d", len(remaining))
		}
	})

	t.Run("returns 404 for non-existent session", func(t *testing.T) {
		truncate(t)
		token := mustAuth(t, "notfound@example.com")

		resp := request(t, http.MethodDelete, "/auth/sessions/00000000-0000-0000-0000-000000000000", nil, token)
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 403 when revoking another user session", func(t *testing.T) {
		truncate(t)
		tok1 := mustAuth(t, "user1@example.com")
		tok2 := mustAuth(t, "user2@example.com")

		listResp := request(t, http.MethodGet, "/auth/sessions", nil, tok1)
		var sessions []struct {
			ID string `json:"id"`
		}
		decodeBody(t, listResp, &sessions)
		if len(sessions) == 0 {
			t.Fatal("expected sessions for user1")
		}

		resp := request(t, http.MethodDelete, "/auth/sessions/"+sessions[0].ID, nil, tok2)
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusForbidden)
	})

	t.Run("rejects request without token", func(t *testing.T) {
		resp := request(t, http.MethodDelete, "/auth/sessions/00000000-0000-0000-0000-000000000000", nil, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusUnauthorized)
	})
}

// ── PATCH /auth/me/avatar ─────────────────────────────────────────────────────

func TestUpdateAvatar(t *testing.T) {
	truncate(t)
	token := mustAuth(t, "avatar@example.com")

	resp := uploadImage(t, http.MethodPatch, "/auth/me/avatar", token)
	assertStatus(t, resp, http.StatusOK)
	var body struct {
		AvatarURL *string `json:"avatar_url"`
	}
	decodeBody(t, resp, &body)
	if body.AvatarURL == nil || *body.AvatarURL == "" {
		t.Fatal("expected avatar_url to be set after upload")
	}

	// The URL is persisted, not just echoed back.
	resp = request(t, http.MethodGet, "/auth/me", nil, token)
	assertStatus(t, resp, http.StatusOK)
	var me struct {
		AvatarURL *string `json:"avatar_url"`
	}
	decodeBody(t, resp, &me)
	if me.AvatarURL == nil || *me.AvatarURL != *body.AvatarURL {
		t.Fatalf("avatar_url not persisted: got %v", me.AvatarURL)
	}
}
