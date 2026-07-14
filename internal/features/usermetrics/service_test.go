package usermetrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LuizFernando991/gym-api/internal/features/task"
	"github.com/LuizFernando991/gym-api/internal/features/workout"
)

type fakeWorkoutService struct {
	items                       []workout.WorkoutDetail
	err                         error
	countCompletedSessions      int
	countCompletedSessionsErr   error
	countCompletedSessionsCalls int
}

func (f fakeWorkoutService) ListWorkouts(ctx context.Context, userID string) ([]workout.WorkoutDetail, error) {
	return f.items, f.err
}

func (f *fakeWorkoutService) CountCompletedSessions(ctx context.Context, userID string, from, to time.Time) (int, error) {
	f.countCompletedSessionsCalls++
	return f.countCompletedSessions, f.countCompletedSessionsErr
}

type fakeTaskService struct {
	items []task.TaskOccurrence
	err   error
}

func (f fakeTaskService) ListByDay(ctx context.Context, userID string, date time.Time) ([]task.TaskOccurrence, error) {
	return f.items, f.err
}

func TestGetTodayMissionsBuildsProgress(t *testing.T) {
	svc := NewService(
		&fakeWorkoutService{
			items: []workout.WorkoutDetail{
				{
					Workout: workout.Workout{
						ID:         "w1",
						Name:       "Push Day",
						DaysOfWeek: workout.DaySlice{workout.Wednesday},
						Active:     true,
					},
					DoneToday: true,
				},
				{
					Workout: workout.Workout{
						ID:         "w2",
						Name:       "Leg Day",
						DaysOfWeek: workout.DaySlice{workout.Wednesday},
						Active:     true,
					},
					DoneToday: false,
				},
				{
					Workout: workout.Workout{
						ID:         "w3",
						Name:       "Inactive",
						DaysOfWeek: workout.DaySlice{workout.Wednesday},
						Active:     false,
					},
					DoneToday: false,
				},
			},
		},
		fakeTaskService{
			items: []task.TaskOccurrence{
				{
					Task: task.Task{
						ID:          "t1",
						Level:       task.LevelEasy,
						Title:       "Drink water",
						IsCompleted: true,
					},
					OccurrenceDate: mustDate(t, "2026-04-22"),
				},
				{
					Task: task.Task{
						ID:          "t2",
						Level:       task.LevelHard,
						Title:       "Read book",
						IsCompleted: false,
					},
					OccurrenceDate: mustDate(t, "2026-04-22"),
				},
			},
		},
	).(*service)

	svc.now = func() time.Time { return mustDate(t, "2026-04-22") }

	resp, err := svc.GetTodayMissions(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetTodayMissions() error = %v", err)
	}

	if resp.Progress.Total != 4 || resp.Progress.Completed != 2 || resp.Progress.Pending != 2 {
		t.Fatalf("overall progress = %+v, want total=4 completed=2 pending=2", resp.Progress)
	}
	if len(resp.Workouts.Items) != 2 {
		t.Fatalf("workout items = %d, want 2", len(resp.Workouts.Items))
	}
	if resp.Workouts.Progress.Total != 2 || resp.Workouts.Progress.Completed != 1 || resp.Workouts.Progress.Pending != 1 {
		t.Fatalf("workout progress = %+v", resp.Workouts.Progress)
	}
	if resp.Tasks.Progress.Total != 2 || resp.Tasks.Progress.Completed != 1 || resp.Tasks.Progress.Pending != 1 {
		t.Fatalf("task progress = %+v", resp.Tasks.Progress)
	}
}

func TestGetTodayMissionsWeekendHasNoWorkoutMissions(t *testing.T) {
	svc := NewService(
		&fakeWorkoutService{
			items: []workout.WorkoutDetail{
				{
					Workout: workout.Workout{
						ID:         "w1",
						Name:       "Push Day",
						DaysOfWeek: workout.DaySlice{workout.Friday},
						Active:     true,
					},
				},
			},
		},
		fakeTaskService{},
	).(*service)

	svc.now = func() time.Time { return mustDate(t, "2026-04-26") }

	resp, err := svc.GetTodayMissions(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetTodayMissions() error = %v", err)
	}
	if len(resp.Workouts.Items) != 0 {
		t.Fatalf("workout items = %d, want 0", len(resp.Workouts.Items))
	}
}

func TestGetTodayMissionsPropagatesErrors(t *testing.T) {
	wantErr := errors.New("boom")
	svc := NewService(&fakeWorkoutService{err: wantErr}, fakeTaskService{}).(*service)
	svc.now = func() time.Time { return mustDate(t, "2026-04-22") }

	_, err := svc.GetTodayMissions(context.Background(), "user-1")
	if !errors.Is(err, wantErr) {
		t.Fatalf("GetTodayMissions() error = %v, want %v", err, wantErr)
	}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return d
}

func TestGetWeeklySummaryCountsScheduledDays(t *testing.T) {
	ws := &fakeWorkoutService{
		items: []workout.WorkoutDetail{
			{
				Workout: workout.Workout{
					ID:         "w1",
					DaysOfWeek: workout.DaySlice{workout.Monday, workout.Wednesday, workout.Friday},
					Active:     true,
				},
			},
			{
				Workout: workout.Workout{
					ID:         "w2",
					DaysOfWeek: workout.DaySlice{workout.Tuesday, workout.Thursday},
					Active:     true,
				},
			},
			{
				Workout: workout.Workout{
					ID:         "w3",
					DaysOfWeek: workout.DaySlice{workout.Saturday},
					Active:     false,
				},
			},
		},
		countCompletedSessions: 4,
	}
	svc := NewService(ws, fakeTaskService{}).(*service)
	svc.now = func() time.Time { return mustDate(t, "2026-04-22") } // Wednesday

	resp, err := svc.GetWeeklySummary(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetWeeklySummary() error = %v", err)
	}
	if resp.Goal.Scheduled != 5 {
		t.Fatalf("scheduled = %d, want 5", resp.Goal.Scheduled)
	}
	if resp.Goal.Completed != 4 {
		t.Fatalf("completed = %d, want 4", resp.Goal.Completed)
	}
	if ws.countCompletedSessionsCalls != 1 {
		t.Fatalf("CountCompletedSessions calls = %d, want 1", ws.countCompletedSessionsCalls)
	}
}

func TestGetTodayMissionsPopulatesDurationAndExerciseCount(t *testing.T) {
	dur := 60
	svc := NewService(
		&fakeWorkoutService{
			items: []workout.WorkoutDetail{
				{
					Workout: workout.Workout{
						ID:         "w1",
						Name:       "Push Day",
						DaysOfWeek: workout.DaySlice{workout.Wednesday},
						Active:     true,
					},
					Exercises: []workout.WorkoutExercise{
						{Sets: 3, Exercise: &workout.Exercise{Type: workout.ExerciseTypeRepetition}},
						{Sets: 2, Exercise: &workout.Exercise{Type: workout.ExerciseTypeTime}, Duration: &dur},
					},
				},
			},
		},
		fakeTaskService{},
	).(*service)
	svc.now = func() time.Time { return mustDate(t, "2026-04-22") }

	resp, err := svc.GetTodayMissions(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("GetTodayMissions() error = %v", err)
	}
	item := resp.Workouts.Items[0]
	if item.ExerciseCount != 2 {
		t.Fatalf("exercise_count = %d, want 2", item.ExerciseCount)
	}
	// 3 sets × 120s = 360s; 2 sets × 60s = 120s; total 480s → 8 min.
	if item.EstimatedDurationMin != 8 {
		t.Fatalf("estimated_duration_min = %d, want 8", item.EstimatedDurationMin)
	}
}
