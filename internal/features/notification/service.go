package notification

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/LuizFernando991/gym-api/internal/infra/push"
)

type Service interface {
	RegisterToken(ctx context.Context, userID, token, platform string) error
	DeleteToken(ctx context.Context, userID, token string) error
	// NotifyWorkoutCompleted notifies co-members when the user logs the day's
	// first completed workout. Fire-and-forget: errors are logged, not returned.
	NotifyWorkoutCompleted(ctx context.Context, userID string, sessionDate time.Time)
}

type service struct {
	repo   Repository
	sender push.Sender
}

func NewService(repo Repository, sender push.Sender) Service {
	return &service{repo: repo, sender: sender}
}

func (s *service) RegisterToken(ctx context.Context, userID, token, platform string) error {
	return s.repo.UpsertToken(ctx, userID, token, platform)
}

func (s *service) DeleteToken(ctx context.Context, userID, token string) error {
	return s.repo.DeleteToken(ctx, userID, token)
}

func (s *service) NotifyWorkoutCompleted(ctx context.Context, userID string, sessionDate time.Time) {
	if err := s.notify(ctx, userID, sessionDate); err != nil {
		slog.Error("notification: notify workout completed", "error", err, "user_id", userID)
	}
}

func (s *service) notify(ctx context.Context, userID string, sessionDate time.Time) error {
	first, err := s.repo.IsFirstCompletionOfDay(ctx, userID, sessionDate)
	if err != nil {
		return err
	}
	if !first {
		return nil // only the first completed workout of the day notifies
	}

	tokens, err := s.repo.CoMemberTokens(ctx, userID)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return nil // no co-members with push tokens
	}

	name, err := s.repo.ActorName(ctx, userID)
	if err != nil {
		return err
	}

	body := fmt.Sprintf("%s treinou 💪", name)
	if err := s.sender.Send(ctx, tokens, "Shadow Leveling", body, map[string]any{"type": "workout"}); err != nil {
		return fmt.Errorf("notification: send push: %w", err)
	}
	return nil
}
