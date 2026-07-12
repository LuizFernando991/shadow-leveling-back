package testutil

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/LuizFernando991/gym-api/internal/features/auth"
)

// FakeVerifier is an auth.TokenVerifier for integration tests. It treats the
// ID token string as a JSON-encoded auth.ProviderClaims, so a test controls the
// asserted identity directly without a real Google/Apple token. A token that
// isn't valid JSON (or lacks a subject) is rejected, standing in for a bad token.
type FakeVerifier struct{}

func (FakeVerifier) Verify(_ context.Context, _ string, idToken string) (*auth.ProviderClaims, error) {
	var claims auth.ProviderClaims
	if err := json.Unmarshal([]byte(idToken), &claims); err != nil {
		return nil, errors.New("fake verifier: not a valid token")
	}
	if claims.Subject == "" {
		return nil, errors.New("fake verifier: missing subject")
	}
	return &claims, nil
}
