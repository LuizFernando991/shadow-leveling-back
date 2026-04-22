package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// Message represents an outgoing email.
// Text carries a plain-text summary used by DevSender (e.g. just the code).
// HTML carries the full rendered body sent by ResendSender.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// Sender is the contract every email backend must satisfy.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// ── Resend adapter ────────────────────────────────────────────────────────────

type resendSender struct {
	apiKey string
	from   string
	client *http.Client
}

// NewResendSender returns a Sender backed by the Resend API.
func NewResendSender(apiKey, from string) Sender {
	return &resendSender{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{},
	}
}

func (s *resendSender) Send(ctx context.Context, msg Message) error {
	payload, err := json.Marshal(map[string]any{
		"from":    s.from,
		"to":      []string{msg.To},
		"subject": msg.Subject,
		"html":    msg.HTML,
	})
	if err != nil {
		return fmt.Errorf("email: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("email: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("email: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("email: resend returned status %d", resp.StatusCode)
	}
	return nil
}

// ── Dev adapter ───────────────────────────────────────────────────────────────

type devSender struct{}

// NewDevSender returns a Sender that prints every message to stdout instead of
// delivering it. Use this in local development so verification codes are
// immediately visible in the terminal without a real email provider.
func NewDevSender() Sender {
	return &devSender{}
}

func (s *devSender) Send(_ context.Context, msg Message) error {
	slog.Info("email (dev)", "to", msg.To, "code", msg.Text)
	return nil
}

// ── Noop adapter ─────────────────────────────────────────────────────────────

type noopSender struct{}

// NewNoopSender returns a Sender that silently discards all messages.
// Intended for tests that read verification codes directly from the database.
func NewNoopSender() Sender {
	return &noopSender{}
}

func (s *noopSender) Send(_ context.Context, _ Message) error {
	return nil
}
