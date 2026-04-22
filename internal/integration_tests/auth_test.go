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
// bypassing the email provider entirely.
func mustGetCode(t *testing.T, emailAddr, vtype string) string {
	t.Helper()
	code, err := testutil.LatestVerificationCode(db, emailAddr, vtype)
	if err != nil {
		t.Fatalf("get verification code: %v", err)
	}
	return code
}

// mustRegister creates a verified user and returns the session token.
func mustRegister(t *testing.T, emailAddr, password string) string {
	t.Helper()

	resp := request(t, http.MethodPost, "/auth/register", map[string]string{
		"email": emailAddr, "password": password,
	}, "")
	if resp.StatusCode != http.StatusCreated {
		resp.Body.Close()
		t.Fatalf("mustRegister: POST /auth/register got %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	code := mustGetCode(t, emailAddr, "register")

	resp2 := request(t, http.MethodPost, "/auth/register/verify", map[string]string{
		"email": emailAddr, "code": code,
	}, "")
	if resp2.StatusCode != http.StatusOK {
		resp2.Body.Close()
		t.Fatalf("mustRegister: POST /auth/register/verify got %d, want 200", resp2.StatusCode)
	}
	var body struct {
		Token string `json:"token"`
	}
	decodeBody(t, resp2, &body)
	return body.Token
}

// mustLogin authenticates a verified user and returns a new session token.
func mustLogin(t *testing.T, emailAddr, password string) string {
	t.Helper()

	resp := request(t, http.MethodPost, "/auth/login", map[string]string{
		"email": emailAddr, "password": password,
	}, "")
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("mustLogin: POST /auth/login got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	code := mustGetCode(t, emailAddr, "login")

	resp2 := request(t, http.MethodPost, "/auth/login/verify", map[string]string{
		"email": emailAddr, "code": code,
	}, "")
	if resp2.StatusCode != http.StatusOK {
		resp2.Body.Close()
		t.Fatalf("mustLogin: POST /auth/login/verify got %d, want 200", resp2.StatusCode)
	}
	var body struct {
		Token string `json:"token"`
	}
	decodeBody(t, resp2, &body)
	return body.Token
}

// ── POST /auth/register ───────────────────────────────────────────────────────

func TestRegister(t *testing.T) {
	t.Run("sends verification code and returns 201", func(t *testing.T) {
		truncate(t)
		resp := request(t, http.MethodPost, "/auth/register", map[string]string{
			"email": "new@example.com", "password": "password123",
		}, "")
		assertStatus(t, resp, http.StatusCreated)

		var body struct{ Message string `json:"message"` }
		decodeBody(t, resp, &body)
		if body.Message == "" {
			t.Error("expected non-empty message")
		}
	})

	t.Run("rejects duplicate email", func(t *testing.T) {
		truncate(t)
		mustRegister(t, "dup@example.com", "password123")

		resp := request(t, http.MethodPost, "/auth/register", map[string]string{
			"email": "dup@example.com", "password": "password123",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusConflict)
	})

	t.Run("rejects invalid email format", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/register", map[string]string{
			"email": "not-an-email", "password": "password123",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects password shorter than 8 characters", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/register", map[string]string{
			"email": "short@example.com", "password": "abc",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects missing email", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/register", map[string]string{
			"password": "password123",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects missing password", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/register", map[string]string{
			"email": "nopass@example.com",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects empty body", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/register", nil, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})
}

// ── POST /auth/register/verify ────────────────────────────────────────────────

func TestVerifyRegistration(t *testing.T) {
	t.Run("returns token on valid code", func(t *testing.T) {
		truncate(t)
		resp := request(t, http.MethodPost, "/auth/register", map[string]string{
			"email": "verify@example.com", "password": "password123",
		}, "")
		resp.Body.Close()

		code := mustGetCode(t, "verify@example.com", "register")
		resp2 := request(t, http.MethodPost, "/auth/register/verify", map[string]string{
			"email": "verify@example.com", "code": code,
		}, "")
		assertStatus(t, resp2, http.StatusOK)

		var body struct {
			Token string `json:"token"`
		}
		decodeBody(t, resp2, &body)
		if body.Token == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("rejects wrong code", func(t *testing.T) {
		truncate(t)
		resp := request(t, http.MethodPost, "/auth/register", map[string]string{
			"email": "wrongcode@example.com", "password": "password123",
		}, "")
		resp.Body.Close()

		resp2 := request(t, http.MethodPost, "/auth/register/verify", map[string]string{
			"email": "wrongcode@example.com", "code": "000000",
		}, "")
		defer resp2.Body.Close()
		assertStatus(t, resp2, http.StatusUnprocessableEntity)
	})

	t.Run("rejects code with non-digit characters", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/register/verify", map[string]string{
			"email": "user@example.com", "code": "abc123",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects code with wrong length", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/register/verify", map[string]string{
			"email": "user@example.com", "code": "123",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("code can only be used once", func(t *testing.T) {
		truncate(t)
		resp := request(t, http.MethodPost, "/auth/register", map[string]string{
			"email": "once@example.com", "password": "password123",
		}, "")
		resp.Body.Close()

		code := mustGetCode(t, "once@example.com", "register")

		resp2 := request(t, http.MethodPost, "/auth/register/verify", map[string]string{
			"email": "once@example.com", "code": code,
		}, "")
		assertStatus(t, resp2, http.StatusOK)
		resp2.Body.Close()

		resp3 := request(t, http.MethodPost, "/auth/register/verify", map[string]string{
			"email": "once@example.com", "code": code,
		}, "")
		defer resp3.Body.Close()
		assertStatus(t, resp3, http.StatusUnprocessableEntity)
	})
}

// ── POST /auth/login ──────────────────────────────────────────────────────────

func TestLogin(t *testing.T) {
	truncate(t)
	mustRegister(t, "login@example.com", "password123")

	t.Run("sends verification code on valid credentials", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/login", map[string]string{
			"email": "login@example.com", "password": "password123",
		}, "")
		assertStatus(t, resp, http.StatusOK)

		var body struct{ Message string `json:"message"` }
		decodeBody(t, resp, &body)
		if body.Message == "" {
			t.Error("expected non-empty message")
		}
	})

	t.Run("rejects wrong password", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/login", map[string]string{
			"email": "login@example.com", "password": "wrongpassword",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("rejects unknown email", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/login", map[string]string{
			"email": "ghost@example.com", "password": "password123",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("rejects unverified user", func(t *testing.T) {
		truncate(t)
		// register but do NOT verify
		resp := request(t, http.MethodPost, "/auth/register", map[string]string{
			"email": "unverified@example.com", "password": "password123",
		}, "")
		resp.Body.Close()

		resp2 := request(t, http.MethodPost, "/auth/login", map[string]string{
			"email": "unverified@example.com", "password": "password123",
		}, "")
		defer resp2.Body.Close()
		assertStatus(t, resp2, http.StatusForbidden)
	})

	t.Run("rejects missing email", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/login", map[string]string{
			"password": "password123",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects missing password", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/login", map[string]string{
			"email": "login@example.com",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects empty body", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/login", nil, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})
}

// ── POST /auth/login/verify ───────────────────────────────────────────────────

func TestVerifyLogin(t *testing.T) {
	t.Run("returns token on valid code", func(t *testing.T) {
		truncate(t)
		mustRegister(t, "loginverify@example.com", "password123")

		resp := request(t, http.MethodPost, "/auth/login", map[string]string{
			"email": "loginverify@example.com", "password": "password123",
		}, "")
		resp.Body.Close()

		code := mustGetCode(t, "loginverify@example.com", "login")
		resp2 := request(t, http.MethodPost, "/auth/login/verify", map[string]string{
			"email": "loginverify@example.com", "code": code,
		}, "")
		assertStatus(t, resp2, http.StatusOK)

		var body struct{ Token string `json:"token"` }
		decodeBody(t, resp2, &body)
		if body.Token == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("rejects wrong code", func(t *testing.T) {
		truncate(t)
		mustRegister(t, "badcode@example.com", "password123")
		resp := request(t, http.MethodPost, "/auth/login", map[string]string{
			"email": "badcode@example.com", "password": "password123",
		}, "")
		resp.Body.Close()

		resp2 := request(t, http.MethodPost, "/auth/login/verify", map[string]string{
			"email": "badcode@example.com", "code": "000000",
		}, "")
		defer resp2.Body.Close()
		assertStatus(t, resp2, http.StatusUnprocessableEntity)
	})

	t.Run("rejects invalid code format", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/login/verify", map[string]string{
			"email": "user@example.com", "code": "ABCDEF",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})
}

// ── GET /auth/me ──────────────────────────────────────────────────────────────

func TestMe(t *testing.T) {
	truncate(t)
	token := mustRegister(t, "me@example.com", "password123")

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
		token := mustRegister(t, "logout@example.com", "password123")

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
		token := mustRegister(t, "sessions@example.com", "password123")
		mustLogin(t, "sessions@example.com", "password123")

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
		token := mustRegister(t, "revcheck@example.com", "password123")
		tok2 := mustLogin(t, "revcheck@example.com", "password123")

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
		token := mustRegister(t, "noip@example.com", "password123")

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
		token := mustRegister(t, "norevokedat@example.com", "password123")

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
		token := mustRegister(t, "revoke@example.com", "password123")
		mustLogin(t, "revoke@example.com", "password123")

		listResp := request(t, http.MethodGet, "/auth/sessions", nil, token)
		var sessions []struct {
			ID string `json:"id"`
		}
		decodeBody(t, listResp, &sessions)
		if len(sessions) != 2 {
			t.Fatalf("expected 2 sessions before revoke, got %d", len(sessions))
		}

		// revoke the newest session (sessions[0] = most recent login)
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
		token := mustRegister(t, "notfound@example.com", "password123")

		resp := request(t, http.MethodDelete, "/auth/sessions/00000000-0000-0000-0000-000000000000", nil, token)
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusNotFound)
	})

	t.Run("returns 403 when revoking another user session", func(t *testing.T) {
		truncate(t)
		tok1 := mustRegister(t, "user1@example.com", "password123")
		tok2 := mustRegister(t, "user2@example.com", "password123")

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
