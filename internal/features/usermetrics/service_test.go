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
	items []workout.WorkoutDetail
	err   error
}

func (f fakeWorkoutService) ListWorkouts(ctx context.Context, userID string) ([]workout.WorkoutDetail, error) {
	return f.items, f.err
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
		fakeWorkoutService{
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
		fakeWorkoutService{
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
	svc := NewService(fakeWorkoutService{err: wantErr}, fakeTaskService{}).(*service)
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
