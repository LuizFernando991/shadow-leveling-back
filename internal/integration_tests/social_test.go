package auth_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

// socialToken builds the fake ID token the test verifier understands: a JSON
// encoding of the claims the provider would assert.
func socialToken(t *testing.T, subject, email string, verified bool) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{
		"subject":        subject,
		"email":          email,
		"email_verified": verified,
	})
	if err != nil {
		t.Fatalf("marshal fake token: %v", err)
	}
	return string(b)
}

func socialLogin(t *testing.T, provider, idToken string) *http.Response {
	t.Helper()
	return request(t, http.MethodPost, "/auth/social", map[string]string{
		"provider": provider, "id_token": idToken,
	}, "")
}

func meEmail(t *testing.T, token string) string {
	t.Helper()
	resp := request(t, http.MethodGet, "/auth/me", nil, token)
	var me struct {
		Email string `json:"email"`
	}
	decodeBody(t, resp, &me)
	return me.Email
}

func TestSocialLogin(t *testing.T) {
	t.Run("creates a verified user for a first-time identity", func(t *testing.T) {
		truncate(t)
		resp := socialLogin(t, "google", socialToken(t, "g-sub-1", "newsocial@example.com", true))
		assertStatus(t, resp, http.StatusOK)

		var body struct {
			Token string `json:"token"`
		}
		decodeBody(t, resp, &body)
		if body.Token == "" {
			t.Fatal("expected non-empty token")
		}
		if got := meEmail(t, body.Token); got != "newsocial@example.com" {
			t.Errorf("me email: got %q, want %q", got, "newsocial@example.com")
		}
	})

	t.Run("same provider subject logs into the same user", func(t *testing.T) {
		truncate(t)
		tok := socialToken(t, "g-sub-2", "repeat@example.com", true)

		first := socialLogin(t, "google", tok)
		assertStatus(t, first, http.StatusOK)
		var b1 struct {
			Token string `json:"token"`
		}
		decodeBody(t, first, &b1)

		second := socialLogin(t, "google", tok)
		assertStatus(t, second, http.StatusOK)
		var b2 struct {
			Token string `json:"token"`
		}
		decodeBody(t, second, &b2)

		if b1.Token == b2.Token {
			t.Error("expected distinct session tokens")
		}
		if meEmail(t, b1.Token) != meEmail(t, b2.Token) {
			t.Error("expected both tokens to resolve to the same user")
		}
	})

	t.Run("links to an existing user with a verified matching email", func(t *testing.T) {
		truncate(t)
		// User first appears via the passwordless email flow.
		emailToken := mustAuth(t, "linkme@example.com")

		// Then signs in with Google using the same, verified email.
		resp := socialLogin(t, "google", socialToken(t, "g-sub-3", "linkme@example.com", true))
		assertStatus(t, resp, http.StatusOK)
		var body struct {
			Token string `json:"token"`
		}
		decodeBody(t, resp, &body)

		// Same account: the sessions list on the original token now includes
		// the social session too (2 sessions for one user).
		listResp := request(t, http.MethodGet, "/auth/sessions", nil, emailToken)
		var sessions []map[string]any
		decodeBody(t, listResp, &sessions)
		if len(sessions) != 2 {
			t.Fatalf("expected 2 sessions on one user after linking, got %d", len(sessions))
		}
	})

	t.Run("does not link when the provider email is unverified", func(t *testing.T) {
		truncate(t)
		mustAuth(t, "unverified-link@example.com")

		// Google asserts the same email but email_verified=false. The security
		// property under test: it must NOT link into the existing account.
		resp := socialLogin(t, "google", socialToken(t, "g-sub-4", "unverified-link@example.com", false))
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Error("unverified provider email must not log into the existing account")
		}
	})

	t.Run("rejects an invalid token", func(t *testing.T) {
		truncate(t)
		resp := socialLogin(t, "google", "not-a-valid-token")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusUnauthorized)
	})

	t.Run("rejects an unsupported provider", func(t *testing.T) {
		resp := socialLogin(t, "facebook", socialToken(t, "f-1", "fb@example.com", true))
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})

	t.Run("rejects a missing id_token", func(t *testing.T) {
		resp := request(t, http.MethodPost, "/auth/social", map[string]string{
			"provider": "google",
		}, "")
		defer resp.Body.Close()
		assertStatus(t, resp, http.StatusBadRequest)
	})
}
