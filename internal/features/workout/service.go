package workout

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/LuizFernando991/gym-api/internal/shared/entities"
)

var (
	ErrNotFound                  = errors.New("not found")
	ErrForbidden                 = errors.New("forbidden")
	ErrInvalidCursor             = errors.New("invalid cursor")
	ErrWorkoutExerciseLimit      = errors.New("workout exercise limit reached")
	ErrInvalidExerciseReordering = errors.New("invalid exercise reordering")
)

const defaultPageSize = 20
const maxWorkoutExercises = 50

type Service interface {
	CreateExercise(ctx context.Context, req CreateExerciseRequest) (*Exercise, error)
	GetExercise(ctx context.Context, id string) (*Exercise, error)
	ListExercises(ctx context.Context, search, cursor string, limit int) (*entities.CursorPage[Exercise], error)

	CreateWorkout(ctx context.Context, userID string, req CreateWorkoutRequest) (*Workout, error)
	GetWorkout(ctx context.Context, id, userID string) (*WorkoutDetail, error)
	ListWorkouts(ctx context.Context, userID string) ([]WorkoutDetail, error)
	UpdateWorkout(ctx context.Context, id, userID string, req UpdateWorkoutRequest) (*Workout, error)
	DeleteWorkout(ctx context.Context, id, userID string) error

	AddWorkoutExercise(ctx context.Context, workoutID, userID string, req AddWorkoutExerciseRequest) (*WorkoutExercise, error)
	UpdateWorkoutExercise(ctx context.Context, weID, workoutID, userID string, req UpdateWorkoutExerciseRequest) (*WorkoutExercise, error)
	DeleteWorkoutExercise(ctx context.Context, weID, workoutID, userID string) error
	ReorderWorkoutExercises(ctx context.Context, workoutID, userID string, req ReorderWorkoutExercisesRequest) error

	CreateSession(ctx context.Context, userID string, req CreateWorkoutSessionRequest) (*WorkoutSession, error)
	GetSession(ctx context.Context, id, userID string) (*WorkoutSessionDetail, error)
	ListSessions(ctx context.Context, userID string, workoutID *string, from, to *time.Time) ([]WorkoutSession, error)
	UpdateSession(ctx context.Context, id, userID string, req UpdateWorkoutSessionRequest) (*WorkoutSession, error)

	RecordSet(ctx context.Context, sessionID, userID string, req RecordSetRequest) (*ExerciseSet, error)
	UpdateSet(ctx context.Context, setID, sessionID, userID string, req UpdateSetRequest) (*ExerciseSet, error)
	DeleteSet(ctx context.Context, setID, sessionID, userID string) error

	GetWorkoutProgress(ctx context.Context, workoutID, userID string, exerciseID *string) ([]ExerciseProgress, error)
	GetMissedSessions(ctx context.Context, userID string, from, to time.Time) ([]MissedSession, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// exerciseCursor holds the position of the last seen exercise for keyset pagination.
type exerciseCursor struct {
	N string `json:"n"` // name
	I string `json:"i"` // id
}

func encodeExerciseCursor(name, id string) string {
	b, _ := json.Marshal(exerciseCursor{N: name, I: id})
	return base64.StdEncoding.EncodeToString(b)
}

func decodeExerciseCursor(cursor string) (name, id string, err error) {
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return "", "", ErrInvalidCursor
	}
	var c exerciseCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return "", "", ErrInvalidCursor
	}
	return c.N, c.I, nil
}

// ── Exercises ──────────────────────────────────────────────────────────────────

func (s *service) CreateExercise(ctx context.Context, req CreateExerciseRequest) (*Exercise, error) {
	e, err := s.repo.CreateExercise(ctx, req.Name, req.Type, req.Unit)
	if err != nil {
		return nil, fmt.Errorf("workout: create exercise: %w", err)
	}
	return e, nil
}

func (s *service) GetExercise(ctx context.Context, id string) (*Exercise, error) {
	e, err := s.repo.GetExercise(ctx, id)
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("workout: get exercise: %w", err)
	}
	return e, nil
}

func (s *service) ListExercises(ctx context.Context, search, cursor string, limit int) (*entities.CursorPage[Exercise], error) {
	if limit <= 0 {
		limit = defaultPageSize
	}

	var afterName, afterID *string
	if cursor != "" {
		n, i, err := decodeExerciseCursor(cursor)
		if err != nil {
			return nil, err
		}
		afterName, afterID = &n, &i
	}

	items, err := s.repo.ListExercises(ctx, search, limit+1, afterName, afterID)
	if err != nil {
		return nil, fmt.Errorf("workout: list exercises: %w", err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	page := &entities.CursorPage[Exercise]{
		Data:   items,
		Cursor: entities.CursorMeta{HasMore: hasMore},
	}
	if page.Data == nil {
		page.Data = []Exercise{}
	}

	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		nc := encodeExerciseCursor(last.Name, last.ID)
		page.Cursor.NextCursor = &nc
	}

	return page, nil
}

// ── Workouts ───────────────────────────────────────────────────────────────────

func (s *service) CreateWorkout(ctx context.Context, userID string, req CreateWorkoutRequest) (*Workout, error) {
	w, err := s.repo.CreateWorkout(ctx, userID, req.Name, req.Description, req.DaysOfWeek)
	if err != nil {
		return nil, fmt.Errorf("workout: create workout: %w", err)
	}
	return w, nil
}

func (s *service) GetWorkout(ctx context.Context, id, userID string) (*WorkoutDetail, error) {
	w, exercises, err := s.repo.GetWorkoutWithExercises(ctx, id)
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("workout: get workout: %w", err)
	}
	if w.UserID != userID {
		return nil, ErrForbidden
	}
	if exercises == nil {
		exercises = []WorkoutExercise{}
	}

	doneToday, err := s.repo.HasCompletedSessionOnDate(ctx, id, time.Now())
	if err != nil {
		return nil, fmt.Errorf("workout: check completed session for today: %w", err)
	}

	return &WorkoutDetail{Workout: *w, Exercises: exercises, DoneToday: doneToday}, nil
}

func (s *service) ListWorkouts(ctx context.Context, userID string) ([]WorkoutDetail, error) {
	workouts, err := s.repo.ListWorkouts(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("workout: list workouts: %w", err)
	}
	return workouts, nil
}

func (s *service) UpdateWorkout(ctx context.Context, id, userID string, req UpdateWorkoutRequest) (*Workout, error) {
	existing, err := s.repo.GetWorkout(ctx, id)
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("workout: get workout: %w", err)
	}
	if existing.UserID != userID {
		return nil, ErrForbidden
	}

	name := existing.Name
	if req.Name != nil {
		name = *req.Name
	}
	description := existing.Description
	if req.Description != nil {
		description = req.Description
	}
	days := existing.DaysOfWeek
	if len(req.DaysOfWeek) > 0 {
		days = req.DaysOfWeek
	}
	active := existing.Active
	if req.Active != nil {
		active = *req.Active
	}

	w, err := s.repo.UpdateWorkout(ctx, id, name, description, days, active)
	if err != nil {
		return nil, fmt.Errorf("workout: update workout: %w", err)
	}
	return w, nil
}

func (s *service) DeleteWorkout(ctx context.Context, id, userID string) error {
	existing, err := s.repo.GetWorkout(ctx, id)
	if isNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("workout: get workout: %w", err)
	}
	if existing.UserID != userID {
		return ErrForbidden
	}
	if err := s.repo.DeleteWorkout(ctx, id); err != nil {
		return fmt.Errorf("workout: delete workout: %w", err)
	}
	return nil
}

// ── WorkoutExercises ───────────────────────────────────────────────────────────

func (s *service) ownsWorkout(ctx context.Context, workoutID, userID string) error {
	w, err := s.repo.GetWorkout(ctx, workoutID)
	if isNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("workout: get workout: %w", err)
	}
	if w.UserID != userID {
		return ErrForbidden
	}
	return nil
}

func (s *service) AddWorkoutExercise(ctx context.Context, workoutID, userID string, req AddWorkoutExerciseRequest) (*WorkoutExercise, error) {
	if err := s.ownsWorkout(ctx, workoutID, userID); err != nil {
		return nil, err
	}
	totalExercises, err := s.repo.CountWorkoutExercises(ctx, workoutID)
	if err != nil {
		return nil, fmt.Errorf("workout: count workout exercises: %w", err)
	}
	if totalExercises >= maxWorkoutExercises {
		return nil, ErrWorkoutExerciseLimit
	}
	we, err := s.repo.AddWorkoutExercise(ctx, workoutID, req.ExerciseID, req.Sets, req.RepsMin, req.RepsMax, req.Duration, req.Note, req.SortOrder)
	if err != nil {
		return nil, fmt.Errorf("workout: add workout exercise: %w", err)
	}
	return we, nil
}

func (s *service) UpdateWorkoutExercise(ctx context.Context, weID, workoutID, userID string, req UpdateWorkoutExerciseRequest) (*WorkoutExercise, error) {
	if err := s.ownsWorkout(ctx, workoutID, userID); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetWorkoutExercise(ctx, weID)
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("workout: get workout exercise: %w", err)
	}
	if existing.WorkoutID != workoutID {
		return nil, ErrNotFound
	}

	sets := existing.Sets
	if req.Sets != nil {
		sets = *req.Sets
	}
	repsMin := existing.RepsMin
	if req.RepsMin != nil {
		repsMin = req.RepsMin
	}
	repsMax := existing.RepsMax
	if req.RepsMax != nil {
		repsMax = req.RepsMax
	}
	duration := existing.Duration
	if req.Duration != nil {
		duration = req.Duration
	}
	note := existing.Note
	if req.Note != nil {
		note = req.Note
	}
	sortOrder := existing.SortOrder
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}

	we, err := s.repo.UpdateWorkoutExercise(ctx, weID, sets, repsMin, repsMax, duration, note, sortOrder)
	if err != nil {
		return nil, fmt.Errorf("workout: update workout exercise: %w", err)
	}
	return we, nil
}

func (s *service) DeleteWorkoutExercise(ctx context.Context, weID, workoutID, userID string) error {
	if err := s.ownsWorkout(ctx, workoutID, userID); err != nil {
		return err
	}
	existing, err := s.repo.GetWorkoutExercise(ctx, weID)
	if isNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("workout: get workout exercise: %w", err)
	}
	if existing.WorkoutID != workoutID {
		return ErrNotFound
	}
	if err := s.repo.DeleteWorkoutExercise(ctx, weID); err != nil {
		return fmt.Errorf("workout: delete workout exercise: %w", err)
	}
	return nil
}

func (s *service) ReorderWorkoutExercises(ctx context.Context, workoutID, userID string, req ReorderWorkoutExercisesRequest) error {
	if err := s.ownsWorkout(ctx, workoutID, userID); err != nil {
		return err
	}

	seenIDs := make(map[string]struct{}, len(req.Exercises))
	orders := make([]WorkoutExerciseOrder, 0, len(req.Exercises))
	for _, exercise := range req.Exercises {
		if _, exists := seenIDs[exercise.ID]; exists {
			return ErrInvalidExerciseReordering
		}
		seenIDs[exercise.ID] = struct{}{}
		orders = append(orders, WorkoutExerciseOrder{
			ID:        exercise.ID,
			SortOrder: exercise.SortOrder,
		})
	}

	if err := s.repo.ReorderWorkoutExercises(ctx, workoutID, orders); isNotFound(err) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("workout: reorder workout exercises: %w", err)
	}

	return nil
}

// ── Sessions ───────────────────────────────────────────────────────────────────

func (s *service) CreateSession(ctx context.Context, userID string, req CreateWorkoutSessionRequest) (*WorkoutSession, error) {
	if err := s.ownsWorkout(ctx, req.WorkoutID, userID); err != nil {
		return nil, err
	}
	sess, err := s.repo.CreateWorkoutSession(ctx, req.WorkoutID, req.Date, req.Status)
	if err != nil {
		return nil, fmt.Errorf("workout: create session: %w", err)
	}
	return sess, nil
}

func (s *service) GetSession(ctx context.Context, id, userID string) (*WorkoutSessionDetail, error) {
	detail, err := s.repo.GetWorkoutSessionDetail(ctx, id)
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("workout: get session: %w", err)
	}
	if err := s.ownsWorkout(ctx, detail.WorkoutID, userID); err != nil {
		return nil, err
	}
	return detail, nil
}

func (s *service) ListSessions(ctx context.Context, userID string, workoutID *string, from, to *time.Time) ([]WorkoutSession, error) {
	sessions, err := s.repo.ListWorkoutSessions(ctx, userID, workoutID, from, to)
	if err != nil {
		return nil, fmt.Errorf("workout: list sessions: %w", err)
	}
	return sessions, nil
}

func (s *service) UpdateSession(ctx context.Context, id, userID string, req UpdateWorkoutSessionRequest) (*WorkoutSession, error) {
	sess, err := s.repo.GetWorkoutSession(ctx, id)
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("workout: get session: %w", err)
	}
	if err := s.ownsWorkout(ctx, sess.WorkoutID, userID); err != nil {
		return nil, err
	}

	status := sess.Status
	if req.Status != nil {
		status = *req.Status
	}

	updated, err := s.repo.UpdateWorkoutSession(ctx, id, status)
	if err != nil {
		return nil, fmt.Errorf("workout: update session: %w", err)
	}
	return updated, nil
}

// ── Sets ───────────────────────────────────────────────────────────────────────

func (s *service) ownsSession(ctx context.Context, sessionID, userID string) error {
	sess, err := s.repo.GetWorkoutSession(ctx, sessionID)
	if isNotFound(err) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("workout: get session: %w", err)
	}
	return s.ownsWorkout(ctx, sess.WorkoutID, userID)
}

func (s *service) RecordSet(ctx context.Context, sessionID, userID string, req RecordSetRequest) (*ExerciseSet, error) {
	if err := s.ownsSession(ctx, sessionID, userID); err != nil {
		return nil, err
	}
	set, err := s.repo.RecordSet(ctx, sessionID, req.ExerciseID, req.SetNumber, req.Reps, req.Weight, req.Duration)
	if err != nil {
		return nil, fmt.Errorf("workout: record set: %w", err)
	}
	return set, nil
}

func (s *service) UpdateSet(ctx context.Context, setID, sessionID, userID string, req UpdateSetRequest) (*ExerciseSet, error) {
	if err := s.ownsSession(ctx, sessionID, userID); err != nil {
		return nil, err
	}
	existing, err := s.repo.GetSet(ctx, setID)
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("workout: get set: %w", err)
	}

	reps := existing.Reps
	if req.Reps != nil {
		reps = req.Reps
	}
	weight := existing.Weight
	if req.Weight != nil {
		weight = req.Weight
	}
	duration := existing.Duration
	if req.Duration != nil {
		duration = req.Duration
	}

	set, err := s.repo.UpdateSet(ctx, setID, reps, weight, duration)
	if err != nil {
		return nil, fmt.Errorf("workout: update set: %w", err)
	}
	return set, nil
}

func (s *service) DeleteSet(ctx context.Context, setID, sessionID, userID string) error {
	if err := s.ownsSession(ctx, sessionID, userID); err != nil {
		return err
	}
	if err := s.repo.DeleteSet(ctx, setID); err != nil {
		return fmt.Errorf("workout: delete set: %w", err)
	}
	return nil
}

// ── Analytics ─────────────────────────────────────────────────────────────────

func (s *service) GetWorkoutProgress(ctx context.Context, workoutID, userID string, exerciseID *string) ([]ExerciseProgress, error) {
	if err := s.ownsWorkout(ctx, workoutID, userID); err != nil {
		return nil, err
	}
	progress, err := s.repo.GetExerciseProgress(ctx, workoutID, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("workout: get progress: %w", err)
	}
	return progress, nil
}

func (s *service) GetMissedSessions(ctx context.Context, userID string, from, to time.Time) ([]MissedSession, error) {
	missed, err := s.repo.GetMissedSessions(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("workout: get missed sessions: %w", err)
	}
	return missed, nil
}
