package leveling

import (
	"context"
	"fmt"
	"time"
)

type Service interface {
	// AwardWorkoutCompletion grants XP for a completed workout session.
	// Idempotent per sessionID.
	AwardWorkoutCompletion(ctx context.Context, userID, sessionID string, sessionDate time.Time) error

	// GetLevelInfo returns the derived progression view for a user.
	GetLevelInfo(ctx context.Context, userID string) (*LevelInfo, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) AwardWorkoutCompletion(ctx context.Context, userID, sessionID string, sessionDate time.Time) error {
	if _, err := s.repo.AwardWorkoutXP(ctx, userID, sessionID, sessionDate); err != nil {
		return fmt.Errorf("leveling: award workout completion: %w", err)
	}
	return nil
}

func (s *service) GetLevelInfo(ctx context.Context, userID string) (*LevelInfo, error) {
	xp, err := s.repo.GetUserXP(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("leveling: get level info: %w", err)
	}
	info := BuildLevelInfo(xp)
	return &info, nil
}