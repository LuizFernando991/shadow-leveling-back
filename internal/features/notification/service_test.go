package notification

import (
	"context"
	"testing"
	"time"
)

type fakeRepo struct {
	first  bool
	tokens []string
	name   string
	err    error
}

func (f fakeRepo) UpsertToken(context.Context, string, string, string) error { return nil }
func (f fakeRepo) DeleteToken(context.Context, string, string) error         { return nil }
func (f fakeRepo) IsFirstCompletionOfDay(context.Context, string, time.Time) (bool, error) {
	return f.first, f.err
}
func (f fakeRepo) CoMemberTokens(context.Context, string) ([]string, error) { return f.tokens, nil }
func (f fakeRepo) ActorName(context.Context, string) (string, error)        { return f.name, nil }

type recordingSender struct {
	calls  int
	tokens []string
	body   string
}

func (r *recordingSender) Send(_ context.Context, tokens []string, _, body string, _ map[string]any) error {
	r.calls++
	r.tokens = tokens
	r.body = body
	return nil
}

func TestNotifyWorkoutCompleted(t *testing.T) {
	ctx := context.Background()

	t.Run("not first of day -> no push", func(t *testing.T) {
		s := &recordingSender{}
		svc := NewService(fakeRepo{first: false, tokens: []string{"t1"}, name: "Sung"}, s)
		svc.NotifyWorkoutCompleted(ctx, "u1", time.Now())
		if s.calls != 0 {
			t.Fatalf("send calls = %d, want 0", s.calls)
		}
	})

	t.Run("first of day with co-members -> push", func(t *testing.T) {
		s := &recordingSender{}
		svc := NewService(fakeRepo{first: true, tokens: []string{"t1", "t2"}, name: "Sung"}, s)
		svc.NotifyWorkoutCompleted(ctx, "u1", time.Now())
		if s.calls != 1 {
			t.Fatalf("send calls = %d, want 1", s.calls)
		}
		if len(s.tokens) != 2 {
			t.Fatalf("tokens = %d, want 2", len(s.tokens))
		}
		if s.body != "Sung treinou 💪" {
			t.Fatalf("body = %q", s.body)
		}
	})

	t.Run("first of day but no co-member tokens -> no push", func(t *testing.T) {
		s := &recordingSender{}
		svc := NewService(fakeRepo{first: true, tokens: nil, name: "Sung"}, s)
		svc.NotifyWorkoutCompleted(ctx, "u1", time.Now())
		if s.calls != 0 {
			t.Fatalf("send calls = %d, want 0", s.calls)
		}
	})
}
