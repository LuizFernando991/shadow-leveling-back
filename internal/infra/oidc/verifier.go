// Package oidc verifies social-provider ID tokens (Google, Apple) against the
// providers' published keys via OIDC discovery. It implements auth.TokenVerifier.
package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/LuizFernando991/gym-api/internal/features/auth"
	"github.com/coreos/go-oidc/v3/oidc"
)

var issuers = map[string]string{
	"google": "https://accounts.google.com",
	"apple":  "https://appleid.apple.com",
}

// Verifier verifies Google/Apple ID tokens, checking signature, issuer, expiry,
// and that the token's audience is one this app accepts. Providers are
// discovered lazily on first use so startup does not depend on Google/Apple
// being reachable.
type Verifier struct {
	auds map[string][]string // provider -> accepted audiences (client IDs)

	mu        sync.Mutex
	verifiers map[string]*oidc.IDTokenVerifier
}

func New(googleAuds, appleAuds []string) *Verifier {
	return &Verifier{
		auds: map[string][]string{
			"google": googleAuds,
			"apple":  appleAuds,
		},
		verifiers: map[string]*oidc.IDTokenVerifier{},
	}
}

func (v *Verifier) Verify(ctx context.Context, provider, rawIDToken string) (*auth.ProviderClaims, error) {
	ver, err := v.verifierFor(ctx, provider)
	if err != nil {
		return nil, err
	}
	// SkipClientIDCheck: go-oidc only accepts a single client ID, but each
	// platform (iOS/Android/web) has its own, so we validate the audience
	// against the accepted set ourselves below.
	tok, err := ver.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("oidc: verify %s token: %w", provider, err)
	}
	if !audienceAllowed(tok.Audience, v.auds[provider]) {
		return nil, fmt.Errorf("oidc: token audience not accepted")
	}

	var raw struct {
		Sub           string          `json:"sub"`
		Email         string          `json:"email"`
		EmailVerified json.RawMessage `json:"email_verified"`
	}
	if err := tok.Claims(&raw); err != nil {
		return nil, fmt.Errorf("oidc: decode claims: %w", err)
	}
	return &auth.ProviderClaims{
		Subject:       raw.Sub,
		Email:         raw.Email,
		EmailVerified: parseBool(raw.EmailVerified),
	}, nil
}

func (v *Verifier) verifierFor(ctx context.Context, provider string) (*oidc.IDTokenVerifier, error) {
	issuer, ok := issuers[provider]
	if !ok {
		return nil, fmt.Errorf("oidc: unsupported provider %q", provider)
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if ver, ok := v.verifiers[provider]; ok {
		return ver, nil
	}
	p, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc: discover %s: %w", provider, err)
	}
	ver := p.Verifier(&oidc.Config{SkipClientIDCheck: true})
	v.verifiers[provider] = ver
	return ver, nil
}

func audienceAllowed(tokenAuds, accepted []string) bool {
	for _, a := range tokenAuds {
		for _, want := range accepted {
			if a == want {
				return true
			}
		}
	}
	return false
}

// parseBool coerces the email_verified claim, which Google sends as a bool and
// Apple as a string ("true"), into a bool.
func parseBool(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		return b
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s == "true"
	}
	return false
}
