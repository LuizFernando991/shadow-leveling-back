package notification

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository interface {
	UpsertToken(ctx context.Context, userID, token, platform string) error
	DeleteToken(ctx context.Context, userID, token string) error
	IsFirstCompletionOfDay(ctx context.Context, userID string, date time.Time) (bool, error)
	CoMemberTokens(ctx context.Context, actorUserID string) ([]string, error)
	ActorName(ctx context.Context, userID string) (string, error)
	SessionOwner(ctx context.Context, sessionID string) (string, error)
	TokensForUser(ctx context.Context, userID string) ([]string, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) UpsertToken(ctx context.Context, userID, token, platform string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO push_tokens (token, user_id, platform, updated_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (token) DO UPDATE
		     SET user_id = EXCLUDED.user_id,
		         platform = EXCLUDED.platform,
		         updated_at = NOW()`,
		token, userID, platform)
	if err != nil {
		return fmt.Errorf("notification: upsert token: %w", err)
	}
	return nil
}

func (r *postgresRepository) DeleteToken(ctx context.Context, userID, token string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM push_tokens WHERE token = $1 AND user_id = $2`, token, userID)
	if err != nil {
		return fmt.Errorf("notification: delete token: %w", err)
	}
	return nil
}

// IsFirstCompletionOfDay reports whether the just-completed session is the
// user's first completed workout on date (the session is already persisted).
func (r *postgresRepository) IsFirstCompletionOfDay(ctx context.Context, userID string, date time.Time) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		   FROM workout_sessions ws
		   JOIN workouts w ON w.id = ws.workout_id
		  WHERE w.user_id = $1 AND ws.status = 'complete' AND ws.date = $2::date`,
		userID, date).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("notification: count completions: %w", err)
	}
	return count == 1, nil
}

// CoMemberTokens returns the push tokens of every distinct user who shares at
// least one group with the actor (excluding the actor).
func (r *postgresRepository) CoMemberTokens(ctx context.Context, actorUserID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT pt.token
		   FROM group_members me
		   JOIN group_members other
		     ON other.group_id = me.group_id AND other.user_id <> me.user_id
		   JOIN push_tokens pt ON pt.user_id = other.user_id
		  WHERE me.user_id = $1`,
		actorUserID)
	if err != nil {
		return nil, fmt.Errorf("notification: co-member tokens: %w", err)
	}
	defer rows.Close()

	tokens := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("notification: scan token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// SessionOwner returns the user id that owns the workout behind a session.
func (r *postgresRepository) SessionOwner(ctx context.Context, sessionID string) (string, error) {
	var ownerID string
	err := r.db.QueryRowContext(ctx,
		`SELECT w.user_id
		   FROM workout_sessions ws
		   JOIN workouts w ON w.id = ws.workout_id
		  WHERE ws.id = $1`,
		sessionID).Scan(&ownerID)
	if err != nil {
		return "", fmt.Errorf("notification: session owner: %w", err)
	}
	return ownerID, nil
}

// TokensForUser returns every push token registered for a single user.
func (r *postgresRepository) TokensForUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT token FROM push_tokens WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("notification: tokens for user: %w", err)
	}
	defer rows.Close()

	tokens := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, fmt.Errorf("notification: scan token: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

func (r *postgresRepository) ActorName(ctx context.Context, userID string) (string, error) {
	var name string
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(nickname, email) FROM users WHERE id = $1`, userID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("notification: actor name: %w", err)
	}
	return name, nil
}
