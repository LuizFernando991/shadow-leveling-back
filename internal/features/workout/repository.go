package workout

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Repository interface {
	CreateExercise(ctx context.Context, name string, etype ExerciseType, unit string) (*Exercise, error)
	GetExercise(ctx context.Context, id string) (*Exercise, error)
	ListExercises(ctx context.Context, search string, limit int, afterName, afterID *string) ([]Exercise, error)

	CreateWorkout(ctx context.Context, userID, name string, description *string, days DaySlice) (*Workout, error)
	GetWorkout(ctx context.Context, id string) (*Workout, error)
	GetWorkoutWithExercises(ctx context.Context, id string) (*Workout, []WorkoutExercise, error)
	ListWorkouts(ctx context.Context, userID string) ([]WorkoutDetail, error)
	UpdateWorkout(ctx context.Context, id, name string, description *string, days DaySlice, active bool) (*Workout, error)
	DeleteWorkout(ctx context.Context, id string) error

	AddWorkoutExercise(ctx context.Context, workoutID, exerciseID string, sets int, repsMin, repsMax, duration *int, note *string, sortOrder int) (*WorkoutExercise, error)
	GetWorkoutExercise(ctx context.Context, id string) (*WorkoutExercise, error)
	UpdateWorkoutExercise(ctx context.Context, id string, sets int, repsMin, repsMax, duration *int, note *string, sortOrder int) (*WorkoutExercise, error)
	DeleteWorkoutExercise(ctx context.Context, id string) error

	CreateWorkoutSession(ctx context.Context, workoutID string, date time.Time, status SessionStatus) (*WorkoutSession, error)
	GetWorkoutSession(ctx context.Context, id string) (*WorkoutSession, error)
	GetWorkoutSessionDetail(ctx context.Context, id string) (*WorkoutSessionDetail, error)
	ListWorkoutSessions(ctx context.Context, userID string, workoutID *string, from, to *time.Time) ([]WorkoutSession, error)
	UpdateWorkoutSession(ctx context.Context, id string, status SessionStatus) (*WorkoutSession, error)

	RecordSet(ctx context.Context, sessionID, exerciseID string, setNumber int, reps *int, weight *float64, duration *int) (*ExerciseSet, error)
	GetSet(ctx context.Context, id string) (*ExerciseSet, error)
	UpdateSet(ctx context.Context, id string, reps *int, weight *float64, duration *int) (*ExerciseSet, error)
	DeleteSet(ctx context.Context, id string) error

	GetExerciseProgress(ctx context.Context, workoutID string, exerciseID *string) ([]ExerciseProgress, error)
	GetMissedSessions(ctx context.Context, userID string, from, to time.Time) ([]MissedSession, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

// ── Exercises ──────────────────────────────────────────────────────────────────

func (r *postgresRepository) CreateExercise(ctx context.Context, name string, etype ExerciseType, unit string) (*Exercise, error) {
	var e Exercise
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO exercises (name, type, unit)
		 VALUES ($1, $2, $3)
		 RETURNING id, name, type, unit, created_at`,
		name, string(etype), unit,
	).Scan(&e.ID, &e.Name, &e.Type, &e.Unit, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: create exercise: %w", err)
	}
	return &e, nil
}

func (r *postgresRepository) GetExercise(ctx context.Context, id string) (*Exercise, error) {
	var e Exercise
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, type, unit, created_at FROM exercises WHERE id = $1`,
		id,
	).Scan(&e.ID, &e.Name, &e.Type, &e.Unit, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: get exercise: %w", err)
	}
	return &e, nil
}

func (r *postgresRepository) ListExercises(ctx context.Context, search string, limit int, afterName, afterID *string) ([]Exercise, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, type, unit, created_at
		 FROM exercises
		 WHERE ($1 = '' OR name ILIKE '%' || $1 || '%')
		   AND ($2::text IS NULL OR name > $2 OR (name = $2 AND id::text > $3))
		 ORDER BY name, id
		 LIMIT $4`,
		search, afterName, afterID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("workout: list exercises: %w", err)
	}
	defer rows.Close()

	var exercises []Exercise
	for rows.Next() {
		var e Exercise
		if err := rows.Scan(&e.ID, &e.Name, &e.Type, &e.Unit, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("workout: scan exercise: %w", err)
		}
		exercises = append(exercises, e)
	}
	return exercises, rows.Err()
}

// ── Workouts ───────────────────────────────────────────────────────────────────

const workoutCols = `id, user_id, name, description, array_to_string(days_of_week, ','), active, created_at, updated_at`

func scanWorkout(s interface{ Scan(...any) error }, w *Workout) error {
	var daysStr string
	if err := s.Scan(&w.ID, &w.UserID, &w.Name, &w.Description, &daysStr, &w.Active, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return err
	}
	w.DaysOfWeek = ParseDaySlice(daysStr)
	return nil
}

func (r *postgresRepository) CreateWorkout(ctx context.Context, userID, name string, description *string, days DaySlice) (*Workout, error) {
	var w Workout
	err := scanWorkout(r.db.QueryRowContext(ctx,
		`INSERT INTO workouts (user_id, name, description, days_of_week)
		 VALUES ($1, $2, $3, $4::text[])
		 RETURNING `+workoutCols,
		userID, name, description, days.PgLiteral(),
	), &w)
	if err != nil {
		return nil, fmt.Errorf("workout: create workout: %w", err)
	}
	return &w, nil
}

func (r *postgresRepository) GetWorkout(ctx context.Context, id string) (*Workout, error) {
	var w Workout
	err := scanWorkout(r.db.QueryRowContext(ctx,
		`SELECT `+workoutCols+` FROM workouts WHERE id = $1`,
		id,
	), &w)
	if err != nil {
		return nil, fmt.Errorf("workout: get workout: %w", err)
	}
	return &w, nil
}

func (r *postgresRepository) GetWorkoutWithExercises(ctx context.Context, id string) (*Workout, []WorkoutExercise, error) {
	w, err := r.GetWorkout(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT we.id, we.workout_id, we.exercise_id,
		        e.id, e.name, e.type, e.unit, e.created_at,
		        we.sets, we.reps_min, we.reps_max, we.duration, we.note, we.sort_order, we.created_at
		 FROM workout_exercises we
		 JOIN exercises e ON e.id = we.exercise_id
		 WHERE we.workout_id = $1
		 ORDER BY we.sort_order, we.created_at`,
		id,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("workout: list workout exercises: %w", err)
	}
	defer rows.Close()

	var exercises []WorkoutExercise
	for rows.Next() {
		var we WorkoutExercise
		var ex Exercise
		if err := rows.Scan(
			&we.ID, &we.WorkoutID, &we.ExerciseID,
			&ex.ID, &ex.Name, &ex.Type, &ex.Unit, &ex.CreatedAt,
			&we.Sets, &we.RepsMin, &we.RepsMax, &we.Duration, &we.Note, &we.SortOrder, &we.CreatedAt,
		); err != nil {
			return nil, nil, fmt.Errorf("workout: scan workout exercise: %w", err)
		}
		we.Exercise = &ex
		exercises = append(exercises, we)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("workout: rows error: %w", err)
	}
	return w, exercises, nil
}

func (r *postgresRepository) ListWorkouts(ctx context.Context, userID string) ([]WorkoutDetail, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+workoutCols+` FROM workouts WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("workout: list workouts: %w", err)
	}
	defer rows.Close()

	var workouts []Workout
	var ids []string
	for rows.Next() {
		var w Workout
		if err := scanWorkout(rows, &w); err != nil {
			return nil, fmt.Errorf("workout: scan workout: %w", err)
		}
		workouts = append(workouts, w)
		ids = append(ids, w.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("workout: rows error: %w", err)
	}

	if len(workouts) == 0 {
		return []WorkoutDetail{}, nil
	}

	exerciseMap, err := r.fetchExercisesForWorkouts(ctx, ids)
	if err != nil {
		return nil, err
	}

	result := make([]WorkoutDetail, len(workouts))
	for i, w := range workouts {
		exs := exerciseMap[w.ID]
		if exs == nil {
			exs = []WorkoutExercise{}
		}
		result[i] = WorkoutDetail{Workout: w, Exercises: exs}
	}
	return result, nil
}

func (r *postgresRepository) fetchExercisesForWorkouts(ctx context.Context, workoutIDs []string) (map[string][]WorkoutExercise, error) {
	placeholders := make([]string, len(workoutIDs))
	args := make([]any, len(workoutIDs))
	for i, id := range workoutIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT we.id, we.workout_id, we.exercise_id,
		        e.id, e.name, e.type, e.unit, e.created_at,
		        we.sets, we.reps_min, we.reps_max, we.duration, we.note, we.sort_order, we.created_at
		 FROM workout_exercises we
		 JOIN exercises e ON e.id = we.exercise_id
		 WHERE we.workout_id IN (`+strings.Join(placeholders, ",")+`)
		 ORDER BY we.workout_id, we.sort_order, we.created_at`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("workout: fetch exercises for workouts: %w", err)
	}
	defer rows.Close()

	exerciseMap := make(map[string][]WorkoutExercise)
	for rows.Next() {
		var we WorkoutExercise
		var ex Exercise
		if err := rows.Scan(
			&we.ID, &we.WorkoutID, &we.ExerciseID,
			&ex.ID, &ex.Name, &ex.Type, &ex.Unit, &ex.CreatedAt,
			&we.Sets, &we.RepsMin, &we.RepsMax, &we.Duration, &we.Note, &we.SortOrder, &we.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("workout: scan workout exercise: %w", err)
		}
		we.Exercise = &ex
		exerciseMap[we.WorkoutID] = append(exerciseMap[we.WorkoutID], we)
	}
	return exerciseMap, rows.Err()
}

func (r *postgresRepository) UpdateWorkout(ctx context.Context, id, name string, description *string, days DaySlice, active bool) (*Workout, error) {
	var w Workout
	err := scanWorkout(r.db.QueryRowContext(ctx,
		`UPDATE workouts
		 SET name = $1, description = $2, days_of_week = $3::text[], active = $4, updated_at = NOW()
		 WHERE id = $5
		 RETURNING `+workoutCols,
		name, description, days.PgLiteral(), active, id,
	), &w)
	if err != nil {
		return nil, fmt.Errorf("workout: update workout: %w", err)
	}
	return &w, nil
}

func (r *postgresRepository) DeleteWorkout(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM workouts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("workout: delete workout: %w", err)
	}
	return nil
}

// ── WorkoutExercises ───────────────────────────────────────────────────────────

func (r *postgresRepository) AddWorkoutExercise(ctx context.Context, workoutID, exerciseID string, sets int, repsMin, repsMax, duration *int, note *string, sortOrder int) (*WorkoutExercise, error) {
	var we WorkoutExercise
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO workout_exercises (workout_id, exercise_id, sets, reps_min, reps_max, duration, note, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, workout_id, exercise_id, sets, reps_min, reps_max, duration, note, sort_order, created_at`,
		workoutID, exerciseID, sets, repsMin, repsMax, duration, note, sortOrder,
	).Scan(&we.ID, &we.WorkoutID, &we.ExerciseID, &we.Sets, &we.RepsMin, &we.RepsMax, &we.Duration, &we.Note, &we.SortOrder, &we.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: add workout exercise: %w", err)
	}
	return &we, nil
}

func (r *postgresRepository) GetWorkoutExercise(ctx context.Context, id string) (*WorkoutExercise, error) {
	var we WorkoutExercise
	err := r.db.QueryRowContext(ctx,
		`SELECT id, workout_id, exercise_id, sets, reps_min, reps_max, duration, note, sort_order, created_at
		 FROM workout_exercises WHERE id = $1`,
		id,
	).Scan(&we.ID, &we.WorkoutID, &we.ExerciseID, &we.Sets, &we.RepsMin, &we.RepsMax, &we.Duration, &we.Note, &we.SortOrder, &we.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: get workout exercise: %w", err)
	}
	return &we, nil
}

func (r *postgresRepository) UpdateWorkoutExercise(ctx context.Context, id string, sets int, repsMin, repsMax, duration *int, note *string, sortOrder int) (*WorkoutExercise, error) {
	var we WorkoutExercise
	err := r.db.QueryRowContext(ctx,
		`UPDATE workout_exercises
		 SET sets = $1, reps_min = $2, reps_max = $3, duration = $4, note = $5, sort_order = $6
		 WHERE id = $7
		 RETURNING id, workout_id, exercise_id, sets, reps_min, reps_max, duration, note, sort_order, created_at`,
		sets, repsMin, repsMax, duration, note, sortOrder, id,
	).Scan(&we.ID, &we.WorkoutID, &we.ExerciseID, &we.Sets, &we.RepsMin, &we.RepsMax, &we.Duration, &we.Note, &we.SortOrder, &we.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: update workout exercise: %w", err)
	}
	return &we, nil
}

func (r *postgresRepository) DeleteWorkoutExercise(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM workout_exercises WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("workout: delete workout exercise: %w", err)
	}
	return nil
}

// ── Sessions ───────────────────────────────────────────────────────────────────

func (r *postgresRepository) CreateWorkoutSession(ctx context.Context, workoutID string, date time.Time, status SessionStatus) (*WorkoutSession, error) {
	var s WorkoutSession
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO workout_sessions (workout_id, date, status)
		 VALUES ($1, $2, $3)
		 RETURNING id, workout_id, date, status, created_at, updated_at`,
		workoutID, date, string(status),
	).Scan(&s.ID, &s.WorkoutID, &s.Date, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: create session: %w", err)
	}
	return &s, nil
}

func (r *postgresRepository) GetWorkoutSession(ctx context.Context, id string) (*WorkoutSession, error) {
	var s WorkoutSession
	err := r.db.QueryRowContext(ctx,
		`SELECT id, workout_id, date, status, created_at, updated_at
		 FROM workout_sessions WHERE id = $1`,
		id,
	).Scan(&s.ID, &s.WorkoutID, &s.Date, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: get session: %w", err)
	}
	return &s, nil
}

func (r *postgresRepository) GetWorkoutSessionDetail(ctx context.Context, id string) (*WorkoutSessionDetail, error) {
	s, err := r.GetWorkoutSession(ctx, id)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, session_id, exercise_id, set_number, reps, weight, duration, created_at
		 FROM exercise_sets WHERE session_id = $1
		 ORDER BY exercise_id, set_number`,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("workout: list sets: %w", err)
	}
	defer rows.Close()

	var sets []ExerciseSet
	for rows.Next() {
		var es ExerciseSet
		if err := rows.Scan(&es.ID, &es.SessionID, &es.ExerciseID, &es.SetNumber, &es.Reps, &es.Weight, &es.Duration, &es.CreatedAt); err != nil {
			return nil, fmt.Errorf("workout: scan set: %w", err)
		}
		sets = append(sets, es)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("workout: rows error: %w", err)
	}

	return &WorkoutSessionDetail{WorkoutSession: *s, Sets: sets}, nil
}

func (r *postgresRepository) ListWorkoutSessions(ctx context.Context, userID string, workoutID *string, from, to *time.Time) ([]WorkoutSession, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ws.id, ws.workout_id, ws.date, ws.status, ws.created_at, ws.updated_at
		 FROM workout_sessions ws
		 JOIN workouts w ON w.id = ws.workout_id
		 WHERE w.user_id = $1
		   AND ($2::uuid IS NULL OR ws.workout_id = $2::uuid)
		   AND ($3::date IS NULL OR ws.date >= $3::date)
		   AND ($4::date IS NULL OR ws.date <= $4::date)
		 ORDER BY ws.date DESC`,
		userID, workoutID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("workout: list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []WorkoutSession
	for rows.Next() {
		var s WorkoutSession
		if err := rows.Scan(&s.ID, &s.WorkoutID, &s.Date, &s.Status, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("workout: scan session: %w", err)
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *postgresRepository) UpdateWorkoutSession(ctx context.Context, id string, status SessionStatus) (*WorkoutSession, error) {
	var s WorkoutSession
	err := r.db.QueryRowContext(ctx,
		`UPDATE workout_sessions
		 SET status = $1, updated_at = NOW()
		 WHERE id = $2
		 RETURNING id, workout_id, date, status, created_at, updated_at`,
		string(status), id,
	).Scan(&s.ID, &s.WorkoutID, &s.Date, &s.Status, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: update session: %w", err)
	}
	return &s, nil
}

// ── Sets ───────────────────────────────────────────────────────────────────────

func (r *postgresRepository) RecordSet(ctx context.Context, sessionID, exerciseID string, setNumber int, reps *int, weight *float64, duration *int) (*ExerciseSet, error) {
	var es ExerciseSet
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO exercise_sets (session_id, exercise_id, set_number, reps, weight, duration)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, session_id, exercise_id, set_number, reps, weight, duration, created_at`,
		sessionID, exerciseID, setNumber, reps, weight, duration,
	).Scan(&es.ID, &es.SessionID, &es.ExerciseID, &es.SetNumber, &es.Reps, &es.Weight, &es.Duration, &es.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: record set: %w", err)
	}
	return &es, nil
}

func (r *postgresRepository) GetSet(ctx context.Context, id string) (*ExerciseSet, error) {
	var es ExerciseSet
	err := r.db.QueryRowContext(ctx,
		`SELECT id, session_id, exercise_id, set_number, reps, weight, duration, created_at
		 FROM exercise_sets WHERE id = $1`,
		id,
	).Scan(&es.ID, &es.SessionID, &es.ExerciseID, &es.SetNumber, &es.Reps, &es.Weight, &es.Duration, &es.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: get set: %w", err)
	}
	return &es, nil
}

func (r *postgresRepository) UpdateSet(ctx context.Context, id string, reps *int, weight *float64, duration *int) (*ExerciseSet, error) {
	var es ExerciseSet
	err := r.db.QueryRowContext(ctx,
		`UPDATE exercise_sets
		 SET reps = $1, weight = $2, duration = $3
		 WHERE id = $4
		 RETURNING id, session_id, exercise_id, set_number, reps, weight, duration, created_at`,
		reps, weight, duration, id,
	).Scan(&es.ID, &es.SessionID, &es.ExerciseID, &es.SetNumber, &es.Reps, &es.Weight, &es.Duration, &es.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("workout: update set: %w", err)
	}
	return &es, nil
}

func (r *postgresRepository) DeleteSet(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM exercise_sets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("workout: delete set: %w", err)
	}
	return nil
}

// ── Analytics ─────────────────────────────────────────────────────────────────

func (r *postgresRepository) GetExerciseProgress(ctx context.Context, workoutID string, exerciseID *string) ([]ExerciseProgress, error) {
	rows, err := r.db.QueryContext(ctx,
		`WITH best_sets AS (
		     SELECT DISTINCT ON (ws.date, es.exercise_id)
		         ws.date,
		         es.id, es.session_id, es.exercise_id, es.set_number, es.reps, es.weight, es.duration, es.created_at
		     FROM exercise_sets es
		     JOIN workout_sessions ws ON ws.id = es.session_id
		     WHERE ws.workout_id = $1
		       AND ws.status = 'complete'
		       AND ($2::uuid IS NULL OR es.exercise_id = $2::uuid)
		     ORDER BY ws.date, es.exercise_id,
		         COALESCE(es.weight, 0) DESC,
		         COALESCE(es.reps, 0) DESC,
		         COALESCE(es.duration, 0) DESC
		 )
		 SELECT e.id, e.name, e.type,
		        bs.date, bs.id, bs.session_id, bs.exercise_id, bs.set_number, bs.reps, bs.weight, bs.duration, bs.created_at
		 FROM best_sets bs
		 JOIN exercises e ON e.id = bs.exercise_id
		 ORDER BY e.name, bs.date`,
		workoutID, exerciseID,
	)
	if err != nil {
		return nil, fmt.Errorf("workout: get exercise progress: %w", err)
	}
	defer rows.Close()

	progressMap := make(map[string]*ExerciseProgress)
	var order []string

	for rows.Next() {
		var exID, exName string
		var exType ExerciseType
		var date time.Time
		var es ExerciseSet

		if err := rows.Scan(
			&exID, &exName, &exType,
			&date, &es.ID, &es.SessionID, &es.ExerciseID, &es.SetNumber, &es.Reps, &es.Weight, &es.Duration, &es.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("workout: scan progress: %w", err)
		}

		if _, exists := progressMap[exID]; !exists {
			progressMap[exID] = &ExerciseProgress{
				ExerciseID:   exID,
				ExerciseName: exName,
				ExerciseType: exType,
			}
			order = append(order, exID)
		}
		progressMap[exID].Sessions = append(progressMap[exID].Sessions, SessionStat{Date: date, BestSet: es})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("workout: rows error: %w", err)
	}

	result := make([]ExerciseProgress, len(order))
	for i, id := range order {
		result[i] = *progressMap[id]
	}
	return result, nil
}

func (r *postgresRepository) GetMissedSessions(ctx context.Context, userID string, from, to time.Time) ([]MissedSession, error) {
	rows, err := r.db.QueryContext(ctx,
		`WITH date_range AS (
		     SELECT generate_series($2::date, $3::date, '1 day'::interval)::date AS day
		 ),
		 scheduled AS (
		     SELECT dr.day, w.id AS workout_id, w.name
		     FROM date_range dr
		     CROSS JOIN workouts w
		     WHERE w.user_id = $1
		       AND w.active = true
		       AND (
		           CASE EXTRACT(DOW FROM dr.day)
		               WHEN 0 THEN 'sunday'
		               WHEN 1 THEN 'monday'
		               WHEN 2 THEN 'tuesday'
		               WHEN 3 THEN 'wednesday'
		               WHEN 4 THEN 'thursday'
		               WHEN 5 THEN 'friday'
		               WHEN 6 THEN 'saturday'
		           END
		       ) = ANY(w.days_of_week)
		 )
		 SELECT s.day, s.workout_id, s.name
		 FROM scheduled s
		 LEFT JOIN workout_sessions ws ON ws.workout_id = s.workout_id
		     AND ws.date = s.day
		     AND ws.status != 'skipped'
		 WHERE ws.id IS NULL
		 ORDER BY s.day DESC`,
		userID, from, to,
	)
	if err != nil {
		return nil, fmt.Errorf("workout: get missed sessions: %w", err)
	}
	defer rows.Close()

	var missed []MissedSession
	for rows.Next() {
		var m MissedSession
		if err := rows.Scan(&m.Date, &m.WorkoutID, &m.WorkoutName); err != nil {
			return nil, fmt.Errorf("workout: scan missed session: %w", err)
		}
		missed = append(missed, m)
	}
	return missed, rows.Err()
}
