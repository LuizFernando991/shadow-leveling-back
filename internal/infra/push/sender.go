package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

const expoPushURL = "https://exp.host/--/api/v2/push/send"
const expoBatchSize = 100

// Sender delivers a push notification to a set of Expo push tokens.
type Sender interface {
	Send(ctx context.Context, tokens []string, title, body string, data map[string]any) error
}

// message is one entry in the Expo push payload.
type message struct {
	To    string         `json:"to"`
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Data  map[string]any `json:"data,omitempty"`
}

// ── Expo adapter ──────────────────────────────────────────────────────────────

type expoSender struct {
	accessToken string // optional; recommended by Expo for extra security
	client      *http.Client
}

// NewExpoSender returns a Sender backed by the Expo push service. accessToken
// may be empty (basic sends work without it).
func NewExpoSender(accessToken string) Sender {
	return &expoSender{accessToken: accessToken, client: &http.Client{}}
}

func (s *expoSender) Send(ctx context.Context, tokens []string, title, body string, data map[string]any) error {
	if len(tokens) == 0 {
		return nil
	}
	for start := 0; start < len(tokens); start += expoBatchSize {
		end := start + expoBatchSize
		if end > len(tokens) {
			end = len(tokens)
		}
		if err := s.sendBatch(ctx, tokens[start:end], title, body, data); err != nil {
			return err
		}
	}
	return nil
}

func (s *expoSender) sendBatch(ctx context.Context, tokens []string, title, body string, data map[string]any) error {
	msgs := make([]message, len(tokens))
	for i, t := range tokens {
		msgs[i] = message{To: t, Title: title, Body: body, Data: data}
	}
	payload, err := json.Marshal(msgs)
	if err != nil {
		return fmt.Errorf("push: marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, expoPushURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("push: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.accessToken)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("push: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("push: expo returned status %d", resp.StatusCode)
	}
	return nil
}

// ── Noop adapter ─────────────────────────────────────────────────────────────

type noopSender struct{}

// NewNoopSender returns a Sender that discards notifications (dev/tests).
func NewNoopSender() Sender { return &noopSender{} }

func (s *noopSender) Send(_ context.Context, tokens []string, title, _ string, _ map[string]any) error {
	slog.Debug("push (noop)", "tokens", len(tokens), "title", title)
	return nil
}
