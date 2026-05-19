package leveling

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository interface {
	// GetUserXP returns the persisted XP state. A user with no row yet
	// returns a zero-value UserXP and found=false.
	GetUserXP(ctx context.Context, userID string) (UserXP, error)

	// AwardWorkoutXP grants XP for completing a workout session, updating
	// the streak, in a single transaction. It is idempotent per session:
	// a second call for the same sessionID is a no-op (returns granted=false).
	AwardWorkoutXP(ctx context.Context, userID, sessionID string, sessionDate time.Time) (granted bool, err error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) GetUserXP(ctx context.Context, userID string) (UserXP, error) {
	var xp UserXP
	err := r.db.QueryRowContext(ctx,
		`SELECT total_xp, current_streak FROM user_xp WHERE user_id = $1`,
		userID,
	).Scan(&xp.TotalXP, &xp.CurrentStreak)
	if err == sql.ErrNoRows {
		return UserXP{}, nil
	}
	if err != nil {
		return UserXP{}, fmt.Errorf("leveling: get user xp: %w", err)
	}
	return xp, nil
}

func (r *postgresRepository) AwardWorkoutXP(ctx context.Context, userID, sessionID string, sessionDate time.Time) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("leveling: begin tx: %w", err)
	}
	defer tx.Rollback()

	// Idempotency guard: the unique partial index on
	// (source_type, source_id) WHERE reason = 'workout_session_completed'
	// makes a second insert for the same session a no-op. RETURNING tells
	// us whether this call actually inserted the row.
	var eventID string
	err = tx.QueryRowContext(ctx,
		`INSERT INTO xp_events (user_id, amount, reason, source_type, source_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (source_type, source_id)
		     WHERE reason = 'workout_session_completed'
		 DO NOTHING
		 RETURNING id`,
		userID, WorkoutCompletionXP, ReasonWorkoutCompleted, SourceTypeWorkoutSession, sessionID,
	).Scan(&eventID)
	if err == sql.ErrNoRows {
		// Already awarded for this session — nothing to do.
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("leveling: insert completion event: %w", err)
	}

	// Load (and lock) the user's XP row; create it on first activity.
	var (
		totalXP    int
		streak     int
		lastActive sql.NullTime
	)
	err = tx.QueryRowContext(ctx,
		`SELECT total_xp, current_streak, last_activity_date
		   FROM user_xp WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&totalXP, &streak, &lastActive)
	if err == sql.ErrNoRows {
		totalXP, streak = 0, 0
		lastActive = sql.NullTime{}
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO user_xp (user_id, total_xp, current_streak) VALUES ($1, 0, 0)`,
			userID,
		); err != nil {
			return false, fmt.Errorf("leveling: init user xp: %w", err)
		}
	} else if err != nil {
		return false, fmt.Errorf("leveling: lock user xp: %w", err)
	}

	sDate := dateOnly(sessionDate)
	newStreak := streak
	advance := true // whether this session moves last_activity_date forward

	switch {
	case !lastActive.Valid:
		newStreak = 1
	default:
		last := dateOnly(lastActive.Time)
		switch {
		case sDate.Equal(last):
			// Same day as last activity: streak unchanged, no advance.
			advance = false
		case sDate.Equal(last.AddDate(0, 0, 1)):
			newStreak = streak + 1
		case sDate.After(last):
			// Gap of 2+ days: streak resets.
			newStreak = 1
		default:
			// Retroactive session (older than last activity): base XP only,
			// streak and last_activity_date untouched.
			advance = false
			newStreak = streak
		}
	}

	bonus := 0
	if advance {
		bonus = StreakBonus(newStreak)
	}
	gain := WorkoutCompletionXP + bonus

	if bonus > 0 {
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO xp_events (user_id, amount, reason, source_type, source_id)
			 VALUES ($1, $2, $3, $4, $5)`,
			userID, bonus, ReasonStreakBonus, SourceTypeWorkoutSession, sessionID,
		); err != nil {
			return false, fmt.Errorf("leveling: insert streak bonus event: %w", err)
		}
	}

	if advance {
		_, err = tx.ExecContext(ctx,
			`UPDATE user_xp
			    SET total_xp = total_xp + $2,
			        current_streak = $3,
			        last_activity_date = $4,
			        updated_at = NOW()
			  WHERE user_id = $1`,
			userID, gain, newStreak, sDate,
		)
	} else {
		_, err = tx.ExecContext(ctx,
			`UPDATE user_xp
			    SET total_xp = total_xp + $2,
			        updated_at = NOW()
			  WHERE user_id = $1`,
			userID, gain,
		)
	}
	if err != nil {
		return false, fmt.Errorf("leveling: apply xp: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("leveling: commit: %w", err)
	}
	return true, nil
}

// dateOnly strips the time component, normalizing to UTC midnight so date
// comparisons match the DATE column semantics.
func dateOnly(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}