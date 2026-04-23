package workout

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestServiceListExercises(t *testing.T) {
	ctx := context.Background()

	t.Run("applies default limit and returns next cursor", func(t *testing.T) {
		repo := &fakeRepository{
			listExercisesResult: makeExercises(21),
		}
		svc := NewService(repo)

		page, err := svc.ListExercises(ctx, "", "", 0)
		if err != nil {
			t.Fatalf("ListExercises() error = %v", err)
		}

		if repo.listExercisesLimit != defaultPageSize+1 {
			t.Fatalf("repo limit = %d, want %d", repo.listExercisesLimit, defaultPageSize+1)
		}
		if len(page.Data) != defaultPageSize {
			t.Fatalf("len(page.Data) = %d, want %d", len(page.Data), defaultPageSize)
		}
		if !page.Cursor.HasMore {
			t.Fatal("HasMore = false, want true")
		}
		if page.Cursor.NextCursor == nil {
			t.Fatal("NextCursor = nil, want cursor")
		}

		name, id, err := decodeExerciseCursor(*page.Cursor.NextCursor)
		if err != nil {
			t.Fatalf("decode next cursor: %v", err)
		}
		last := page.Data[len(page.Data)-1]
		if name != last.Name || id != last.ID {
			t.Fatalf("decoded cursor = (%q, %q), want (%q, %q)", name, id, last.Name, last.ID)
		}
	})

	t.Run("passes decoded cursor to repository", func(t *testing.T) {
		repo := &fakeRepository{}
		svc := NewService(repo)
		cursor := encodeExerciseCursor("Bench Press", "exercise-1")

		_, err := svc.ListExercises(ctx, "bench", cursor, 10)
		if err != nil {
			t.Fatalf("ListExercises() error = %v", err)
		}

		if repo.listExercisesSearch != "bench" {
			t.Fatalf("search = %q, want bench", repo.listExercisesSearch)
		}
		if repo.listExercisesAfterName == nil || *repo.listExercisesAfterName != "Bench Press" {
			t.Fatalf("afterName = %v, want Bench Press", repo.listExercisesAfterName)
		}
		if repo.listExercisesAfterID == nil || *repo.listExercisesAfterID != "exercise-1" {
			t.Fatalf("afterID = %v, want exercise-1", repo.listExercisesAfterID)
		}
	})

	t.Run("rejects invalid cursor", func(t *testing.T) {
		svc := NewService(&fakeRepository{})

		_, err := svc.ListExercises(ctx, "", "not-base64", 10)
		if !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("ListExercises() error = %v, want %v", err, ErrInvalidCursor)
		}
	})

	t.Run("normalizes nil data to empty slice", func(t *testing.T) {
		svc := NewService(&fakeRepository{})

		page, err := svc.ListExercises(ctx, "", "", 10)
		if err != nil {
			t.Fatalf("ListExercises() error = %v", err)
		}
		if page.Data == nil {
			t.Fatal("Data = nil, want empty slice")
		}
	})
}

func TestServiceWorkoutOwnership(t *testing.T) {
	ctx := context.Background()

	t.Run("GetWorkout returns forbidden for another user's workout", func(t *testing.T) {
		repo := &fakeRepository{
			workout: &Workout{ID: "workout-1", UserID: "owner"},
		}
		svc := NewService(repo)

		_, err := svc.GetWorkout(ctx, "workout-1", "intruder")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("GetWorkout() error = %v, want %v", err, ErrForbidden)
		}
		if repo.hasCompletedSessionCalls != 0 {
			t.Fatalf("HasCompletedSessionOnDate calls = %d, want 0", repo.hasCompletedSessionCalls)
		}
	})

	t.Run("GetWorkout maps missing workout to ErrNotFound", func(t *testing.T) {
		repo := &fakeRepository{getWorkoutErr: sql.ErrNoRows}
		svc := NewService(repo)

		_, err := svc.GetWorkout(ctx, "missing", "user-1")
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("GetWorkout() error = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("DeleteWorkout refuses to delete another user's workout", func(t *testing.T) {
		repo := &fakeRepository{
			workout: &Workout{ID: "workout-1", UserID: "owner"},
		}
		svc := NewService(repo)

		err := svc.DeleteWorkout(ctx, "workout-1", "intruder")
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("DeleteWorkout() error = %v, want %v", err, ErrForbidden)
		}
		if repo.deleteWorkoutCalls != 0 {
			t.Fatalf("DeleteWorkout repo calls = %d, want 0", repo.deleteWorkoutCalls)
		}
	})
}

func TestServiceUpdateWorkout(t *testing.T) {
	ctx := context.Background()
	description := "current description"
	existing := &Workout{
		ID:          "workout-1",
		UserID:      "user-1",
		Name:        "Current",
		Description: &description,
		DaysOfWeek:  DaySlice{Monday, Wednesday},
		Active:      true,
	}
	repo := &fakeRepository{workout: existing}
	svc := NewService(repo)

	newName := "Updated"
	active := false
	got, err := svc.UpdateWorkout(ctx, "workout-1", "user-1", UpdateWorkoutRequest{
		Name:   &newName,
		Active: &active,
	})
	if err != nil {
		t.Fatalf("UpdateWorkout() error = %v", err)
	}

	if got.Name != newName {
		t.Fatalf("Name = %q, want %q", got.Name, newName)
	}
	if got.Description == nil || *got.Description != description {
		t.Fatalf("Description = %v, want %q", got.Description, description)
	}
	if !reflect.DeepEqual(got.DaysOfWeek, existing.DaysOfWeek) {
		t.Fatalf("DaysOfWeek = %v, want %v", got.DaysOfWeek, existing.DaysOfWeek)
	}
	if got.Active {
		t.Fatal("Active = true, want false")
	}
}

func TestServiceWorkoutExercises(t *testing.T) {
	ctx := context.Background()

	t.Run("AddWorkoutExercise blocks at exercise limit", func(t *testing.T) {
		repo := &fakeRepository{
			workout:               &Workout{ID: "workout-1", UserID: "user-1"},
			countWorkoutExercises: maxWorkoutExercises,
		}
		svc := NewService(repo)

		_, err := svc.AddWorkoutExercise(ctx, "workout-1", "user-1", AddWorkoutExerciseRequest{
			ExerciseID: "exercise-1",
			Sets:       3,
		})
		if !errors.Is(err, ErrWorkoutExerciseLimit) {
			t.Fatalf("AddWorkoutExercise() error = %v, want %v", err, ErrWorkoutExerciseLimit)
		}
		if repo.addWorkoutExerciseCalls != 0 {
			t.Fatalf("AddWorkoutExercise repo calls = %d, want 0", repo.addWorkoutExerciseCalls)
		}
	})

	t.Run("UpdateWorkoutExercise preserves omitted fields", func(t *testing.T) {
		repsMin := 8
		repsMax := 12
		duration := 60
		note := "slow eccentric"
		existing := &WorkoutExercise{
			ID:        "we-1",
			WorkoutID: "workout-1",
			Sets:      4,
			RepsMin:   &repsMin,
			RepsMax:   &repsMax,
			Duration:  &duration,
			Note:      &note,
			SortOrder: 2,
		}
		repo := &fakeRepository{
			workout:         &Workout{ID: "workout-1", UserID: "user-1"},
			workoutExercise: existing,
		}
		svc := NewService(repo)
		sets := 5

		got, err := svc.UpdateWorkoutExercise(ctx, "we-1", "workout-1", "user-1", UpdateWorkoutExerciseRequest{
			Sets: &sets,
		})
		if err != nil {
			t.Fatalf("UpdateWorkoutExercise() error = %v", err)
		}

		if got.Sets != sets || got.RepsMin != &repsMin || got.RepsMax != &repsMax || got.Duration != &duration || got.Note != &note || got.SortOrder != 2 {
			t.Fatalf("updated workout exercise = %+v, omitted fields were not preserved", got)
		}
	})

	t.Run("UpdateWorkoutExercise treats mismatched workout as not found", func(t *testing.T) {
		repo := &fakeRepository{
			workout:         &Workout{ID: "workout-1", UserID: "user-1"},
			workoutExercise: &WorkoutExercise{ID: "we-1", WorkoutID: "other-workout"},
		}
		svc := NewService(repo)

		_, err := svc.UpdateWorkoutExercise(ctx, "we-1", "workout-1", "user-1", UpdateWorkoutExerciseRequest{})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("UpdateWorkoutExercise() error = %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("ReorderWorkoutExercises rejects duplicates before repository update", func(t *testing.T) {
		repo := &fakeRepository{workout: &Workout{ID: "workout-1", UserID: "user-1"}}
		svc := NewService(repo)

		err := svc.ReorderWorkoutExercises(ctx, "workout-1", "user-1", ReorderWorkoutExercisesRequest{
			Exercises: []ReorderWorkoutExerciseItem{
				{ID: "we-1", SortOrder: 1},
				{ID: "we-1", SortOrder: 2},
			},
		})
		if !errors.Is(err, ErrInvalidExerciseReordering) {
			t.Fatalf("ReorderWorkoutExercises() error = %v, want %v", err, ErrInvalidExerciseReordering)
		}
		if repo.reorderWorkoutExercisesCalls != 0 {
			t.Fatalf("ReorderWorkoutExercises repo calls = %d, want 0", repo.reorderWorkoutExercisesCalls)
		}
	})
}

func TestServiceSessionsAndSets(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateSession checks workout ownership", func(t *testing.T) {
		repo := &fakeRepository{workout: &Workout{ID: "workout-1", UserID: "owner"}}
		svc := NewService(repo)

		_, err := svc.CreateSession(ctx, "intruder", CreateWorkoutSessionRequest{
			WorkoutID: "workout-1",
			Date:      time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC),
			Status:    StatusComplete,
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("CreateSession() error = %v, want %v", err, ErrForbidden)
		}
		if repo.createWorkoutSessionCalls != 0 {
			t.Fatalf("CreateWorkoutSession repo calls = %d, want 0", repo.createWorkoutSessionCalls)
		}
	})

	t.Run("UpdateSession preserves existing status when omitted", func(t *testing.T) {
		repo := &fakeRepository{
			workout:        &Workout{ID: "workout-1", UserID: "user-1"},
			workoutSession: &WorkoutSession{ID: "session-1", WorkoutID: "workout-1", Status: StatusIncomplete},
		}
		svc := NewService(repo)

		got, err := svc.UpdateSession(ctx, "session-1", "user-1", UpdateWorkoutSessionRequest{})
		if err != nil {
			t.Fatalf("UpdateSession() error = %v", err)
		}
		if got.Status != StatusIncomplete {
			t.Fatalf("Status = %q, want %q", got.Status, StatusIncomplete)
		}
	})

	t.Run("RecordSet checks session ownership", func(t *testing.T) {
		repo := &fakeRepository{
			workout:        &Workout{ID: "workout-1", UserID: "owner"},
			workoutSession: &WorkoutSession{ID: "session-1", WorkoutID: "workout-1"},
		}
		svc := NewService(repo)

		_, err := svc.RecordSet(ctx, "session-1", "intruder", RecordSetRequest{
			ExerciseID: "exercise-1",
			SetNumber:  1,
		})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("RecordSet() error = %v, want %v", err, ErrForbidden)
		}
		if repo.recordSetCalls != 0 {
			t.Fatalf("RecordSet repo calls = %d, want 0", repo.recordSetCalls)
		}
	})

	t.Run("UpdateSet preserves omitted fields", func(t *testing.T) {
		reps := 10
		weight := 42.5
		duration := 30
		repo := &fakeRepository{
			workout:        &Workout{ID: "workout-1", UserID: "user-1"},
			workoutSession: &WorkoutSession{ID: "session-1", WorkoutID: "workout-1"},
			exerciseSet:    &ExerciseSet{ID: "set-1", SessionID: "session-1", Reps: &reps, Weight: &weight, Duration: &duration},
		}
		svc := NewService(repo)
		newReps := 12

		got, err := svc.UpdateSet(ctx, "set-1", "session-1", "user-1", UpdateSetRequest{Reps: &newReps})
		if err != nil {
			t.Fatalf("UpdateSet() error = %v", err)
		}
		if got.Reps != &newReps || got.Weight != &weight || got.Duration != &duration {
			t.Fatalf("updated set = %+v, omitted fields were not preserved", got)
		}
	})
}

func TestServiceAnalytics(t *testing.T) {
	ctx := context.Background()

	t.Run("GetWorkoutProgress checks workout ownership", func(t *testing.T) {
		repo := &fakeRepository{workout: &Workout{ID: "workout-1", UserID: "owner"}}
		svc := NewService(repo)

		_, err := svc.GetWorkoutProgress(ctx, "workout-1", "intruder", nil)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("GetWorkoutProgress() error = %v, want %v", err, ErrForbidden)
		}
		if repo.getExerciseProgressCalls != 0 {
			t.Fatalf("GetExerciseProgress repo calls = %d, want 0", repo.getExerciseProgressCalls)
		}
	})

	t.Run("GetMissedSessions delegates to repository", func(t *testing.T) {
		from := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
		to := time.Date(2026, 4, 23, 0, 0, 0, 0, time.UTC)
		want := []MissedSession{{WorkoutID: "workout-1", WorkoutName: "Push", Date: from}}
		repo := &fakeRepository{missedSessions: want}
		svc := NewService(repo)

		got, err := svc.GetMissedSessions(ctx, "user-1", from, to)
		if err != nil {
			t.Fatalf("GetMissedSessions() error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("GetMissedSessions() = %v, want %v", got, want)
		}
		if repo.missedSessionsUserID != "user-1" || !repo.missedSessionsFrom.Equal(from) || !repo.missedSessionsTo.Equal(to) {
			t.Fatalf("repository received user=%q from=%v to=%v", repo.missedSessionsUserID, repo.missedSessionsFrom, repo.missedSessionsTo)
		}
	})
}

func makeExercises(n int) []Exercise {
	exercises := make([]Exercise, n)
	for i := range exercises {
		exercises[i] = Exercise{
			ID:   "exercise-" + string(rune('a'+i)),
			Name: "Exercise " + string(rune('A'+i)),
			Type: ExerciseTypeRepetition,
			Unit: "reps",
		}
	}
	return exercises
}

type fakeRepository struct {
	createExerciseResult *Exercise
	createExerciseErr    error

	exercise    *Exercise
	exerciseErr error

	listExercisesResult    []Exercise
	listExercisesErr       error
	listExercisesSearch    string
	listExercisesLimit     int
	listExercisesAfterName *string
	listExercisesAfterID   *string

	createWorkoutResult *Workout
	createWorkoutErr    error

	workout       *Workout
	getWorkoutErr error

	workoutExercises   []WorkoutExercise
	getWorkoutWithErr  error
	listWorkoutsResult []WorkoutDetail
	listWorkoutsErr    error

	hasCompletedSessionResult bool
	hasCompletedSessionErr    error
	hasCompletedSessionCalls  int

	updateWorkoutErr   error
	deleteWorkoutErr   error
	deleteWorkoutCalls int

	countWorkoutExercises        int
	countWorkoutExercisesErr     error
	workoutExercise              *WorkoutExercise
	getWorkoutExerciseErr        error
	addWorkoutExerciseErr        error
	addWorkoutExerciseCalls      int
	updateWorkoutExerciseErr     error
	deleteWorkoutExerciseErr     error
	reorderWorkoutExercisesErr   error
	reorderWorkoutExercisesCalls int

	workoutSession             *WorkoutSession
	getWorkoutSessionErr       error
	workoutSessionDetail       *WorkoutSessionDetail
	getWorkoutSessionDetailErr error
	createWorkoutSessionErr    error
	createWorkoutSessionCalls  int
	listWorkoutSessionsResult  []WorkoutSession
	listWorkoutSessionsErr     error
	updateWorkoutSessionErr    error

	exerciseSet    *ExerciseSet
	getSetErr      error
	recordSetErr   error
	recordSetCalls int
	updateSetErr   error
	deleteSetErr   error

	exerciseProgress         []ExerciseProgress
	getExerciseProgressErr   error
	getExerciseProgressCalls int

	missedSessions       []MissedSession
	getMissedSessionsErr error
	missedSessionsUserID string
	missedSessionsFrom   time.Time
	missedSessionsTo     time.Time
}

func (r *fakeRepository) CreateExercise(ctx context.Context, name string, etype ExerciseType, unit string) (*Exercise, error) {
	if r.createExerciseErr != nil {
		return nil, r.createExerciseErr
	}
	if r.createExerciseResult != nil {
		return r.createExerciseResult, nil
	}
	return &Exercise{Name: name, Type: etype, Unit: unit}, nil
}

func (r *fakeRepository) GetExercise(ctx context.Context, id string) (*Exercise, error) {
	if r.exerciseErr != nil {
		return nil, r.exerciseErr
	}
	if r.exercise != nil {
		return r.exercise, nil
	}
	return &Exercise{ID: id}, nil
}

func (r *fakeRepository) ListExercises(ctx context.Context, search string, limit int, afterName, afterID *string) ([]Exercise, error) {
	r.listExercisesSearch = search
	r.listExercisesLimit = limit
	r.listExercisesAfterName = afterName
	r.listExercisesAfterID = afterID
	if r.listExercisesErr != nil {
		return nil, r.listExercisesErr
	}
	return r.listExercisesResult, nil
}

func (r *fakeRepository) CreateWorkout(ctx context.Context, userID, name string, description *string, days DaySlice) (*Workout, error) {
	if r.createWorkoutErr != nil {
		return nil, r.createWorkoutErr
	}
	if r.createWorkoutResult != nil {
		return r.createWorkoutResult, nil
	}
	return &Workout{UserID: userID, Name: name, Description: description, DaysOfWeek: days}, nil
}

func (r *fakeRepository) GetWorkout(ctx context.Context, id string) (*Workout, error) {
	if r.getWorkoutErr != nil {
		return nil, r.getWorkoutErr
	}
	if r.workout != nil {
		return r.workout, nil
	}
	return &Workout{ID: id}, nil
}

func (r *fakeRepository) GetWorkoutWithExercises(ctx context.Context, id string) (*Workout, []WorkoutExercise, error) {
	if r.getWorkoutWithErr != nil {
		return nil, nil, r.getWorkoutWithErr
	}
	if r.getWorkoutErr != nil {
		return nil, nil, r.getWorkoutErr
	}
	if r.workout != nil {
		return r.workout, r.workoutExercises, nil
	}
	return &Workout{ID: id}, r.workoutExercises, nil
}

func (r *fakeRepository) ListWorkouts(ctx context.Context, userID string) ([]WorkoutDetail, error) {
	if r.listWorkoutsErr != nil {
		return nil, r.listWorkoutsErr
	}
	return r.listWorkoutsResult, nil
}

func (r *fakeRepository) HasCompletedSessionOnDate(ctx context.Context, workoutID string, date time.Time) (bool, error) {
	r.hasCompletedSessionCalls++
	if r.hasCompletedSessionErr != nil {
		return false, r.hasCompletedSessionErr
	}
	return r.hasCompletedSessionResult, nil
}

func (r *fakeRepository) UpdateWorkout(ctx context.Context, id, name string, description *string, days DaySlice, active bool) (*Workout, error) {
	if r.updateWorkoutErr != nil {
		return nil, r.updateWorkoutErr
	}
	return &Workout{ID: id, UserID: r.workout.UserID, Name: name, Description: description, DaysOfWeek: days, Active: active}, nil
}

func (r *fakeRepository) DeleteWorkout(ctx context.Context, id string) error {
	r.deleteWorkoutCalls++
	return r.deleteWorkoutErr
}

func (r *fakeRepository) AddWorkoutExercise(ctx context.Context, workoutID, exerciseID string, sets int, repsMin, repsMax, duration *int, note *string, sortOrder int) (*WorkoutExercise, error) {
	r.addWorkoutExerciseCalls++
	if r.addWorkoutExerciseErr != nil {
		return nil, r.addWorkoutExerciseErr
	}
	return &WorkoutExercise{WorkoutID: workoutID, ExerciseID: exerciseID, Sets: sets, RepsMin: repsMin, RepsMax: repsMax, Duration: duration, Note: note, SortOrder: sortOrder}, nil
}

func (r *fakeRepository) CountWorkoutExercises(ctx context.Context, workoutID string) (int, error) {
	return r.countWorkoutExercises, r.countWorkoutExercisesErr
}

func (r *fakeRepository) GetWorkoutExercise(ctx context.Context, id string) (*WorkoutExercise, error) {
	if r.getWorkoutExerciseErr != nil {
		return nil, r.getWorkoutExerciseErr
	}
	if r.workoutExercise != nil {
		return r.workoutExercise, nil
	}
	return &WorkoutExercise{ID: id}, nil
}

func (r *fakeRepository) UpdateWorkoutExercise(ctx context.Context, id string, sets int, repsMin, repsMax, duration *int, note *string, sortOrder int) (*WorkoutExercise, error) {
	if r.updateWorkoutExerciseErr != nil {
		return nil, r.updateWorkoutExerciseErr
	}
	return &WorkoutExercise{ID: id, WorkoutID: r.workoutExercise.WorkoutID, Sets: sets, RepsMin: repsMin, RepsMax: repsMax, Duration: duration, Note: note, SortOrder: sortOrder}, nil
}

func (r *fakeRepository) DeleteWorkoutExercise(ctx context.Context, id string) error {
	return r.deleteWorkoutExerciseErr
}

func (r *fakeRepository) ReorderWorkoutExercises(ctx context.Context, workoutID string, orders []WorkoutExerciseOrder) error {
	r.reorderWorkoutExercisesCalls++
	return r.reorderWorkoutExercisesErr
}

func (r *fakeRepository) CreateWorkoutSession(ctx context.Context, workoutID string, date time.Time, status SessionStatus) (*WorkoutSession, error) {
	r.createWorkoutSessionCalls++
	if r.createWorkoutSessionErr != nil {
		return nil, r.createWorkoutSessionErr
	}
	return &WorkoutSession{WorkoutID: workoutID, Date: date, Status: status}, nil
}

func (r *fakeRepository) GetWorkoutSession(ctx context.Context, id string) (*WorkoutSession, error) {
	if r.getWorkoutSessionErr != nil {
		return nil, r.getWorkoutSessionErr
	}
	if r.workoutSession != nil {
		return r.workoutSession, nil
	}
	return &WorkoutSession{ID: id}, nil
}

func (r *fakeRepository) GetWorkoutSessionDetail(ctx context.Context, id string) (*WorkoutSessionDetail, error) {
	if r.getWorkoutSessionDetailErr != nil {
		return nil, r.getWorkoutSessionDetailErr
	}
	if r.workoutSessionDetail != nil {
		return r.workoutSessionDetail, nil
	}
	return &WorkoutSessionDetail{WorkoutSession: WorkoutSession{ID: id}}, nil
}

func (r *fakeRepository) ListWorkoutSessions(ctx context.Context, userID string, workoutID *string, from, to *time.Time) ([]WorkoutSession, error) {
	if r.listWorkoutSessionsErr != nil {
		return nil, r.listWorkoutSessionsErr
	}
	return r.listWorkoutSessionsResult, nil
}

func (r *fakeRepository) UpdateWorkoutSession(ctx context.Context, id string, status SessionStatus) (*WorkoutSession, error) {
	if r.updateWorkoutSessionErr != nil {
		return nil, r.updateWorkoutSessionErr
	}
	return &WorkoutSession{ID: id, WorkoutID: r.workoutSession.WorkoutID, Status: status}, nil
}

func (r *fakeRepository) RecordSet(ctx context.Context, sessionID, exerciseID string, setNumber int, reps *int, weight *float64, duration *int) (*ExerciseSet, error) {
	r.recordSetCalls++
	if r.recordSetErr != nil {
		return nil, r.recordSetErr
	}
	return &ExerciseSet{SessionID: sessionID, ExerciseID: exerciseID, SetNumber: setNumber, Reps: reps, Weight: weight, Duration: duration}, nil
}

func (r *fakeRepository) GetSet(ctx context.Context, id string) (*ExerciseSet, error) {
	if r.getSetErr != nil {
		return nil, r.getSetErr
	}
	if r.exerciseSet != nil {
		return r.exerciseSet, nil
	}
	return &ExerciseSet{ID: id}, nil
}

func (r *fakeRepository) UpdateSet(ctx context.Context, id string, reps *int, weight *float64, duration *int) (*ExerciseSet, error) {
	if r.updateSetErr != nil {
		return nil, r.updateSetErr
	}
	return &ExerciseSet{ID: id, Reps: reps, Weight: weight, Duration: duration}, nil
}

func (r *fakeRepository) DeleteSet(ctx context.Context, id string) error {
	return r.deleteSetErr
}

func (r *fakeRepository) GetExerciseProgress(ctx context.Context, workoutID string, exerciseID *string) ([]ExerciseProgress, error) {
	r.getExerciseProgressCalls++
	if r.getExerciseProgressErr != nil {
		return nil, r.getExerciseProgressErr
	}
	return r.exerciseProgress, nil
}

func (r *fakeRepository) GetMissedSessions(ctx context.Context, userID string, from, to time.Time) ([]MissedSession, error) {
	r.missedSessionsUserID = userID
	r.missedSessionsFrom = from
	r.missedSessionsTo = to
	if r.getMissedSessionsErr != nil {
		return nil, r.getMissedSessionsErr
	}
	return r.missedSessions, nil
}
