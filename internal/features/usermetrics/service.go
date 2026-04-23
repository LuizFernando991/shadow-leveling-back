package usermetrics

import (
	"context"
	"time"

	"github.com/LuizFernando991/gym-api/internal/features/task"
	"github.com/LuizFernando991/gym-api/internal/features/workout"
)

type WorkoutService interface {
	ListWorkouts(ctx context.Context, userID string) ([]workout.WorkoutDetail, error)
}

type TaskService interface {
	ListByDay(ctx context.Context, userID string, date time.Time) ([]task.TaskOccurrence, error)
}

type Service interface {
	GetTodayMissions(ctx context.Context, userID string) (*TodayMissionsResponse, error)
}

type service struct {
	workoutSvc WorkoutService
	taskSvc    TaskService
	now        func() time.Time
}

func NewService(workoutSvc WorkoutService, taskSvc TaskService) Service {
	return &service{
		workoutSvc: workoutSvc,
		taskSvc:    taskSvc,
		now:        time.Now,
	}
}

func (s *service) GetTodayMissions(ctx context.Context, userID string) (*TodayMissionsResponse, error) {
	today := dateOnly(s.now())

	workouts, err := s.workoutSvc.ListWorkouts(ctx, userID)
	if err != nil {
		return nil, err
	}

	taskOccurrences, err := s.taskSvc.ListByDay(ctx, userID, today)
	if err != nil {
		return nil, err
	}

	workoutItems := scheduledWorkoutsForDay(workouts, today)
	taskItems := make([]TaskMission, len(taskOccurrences))
	for i, occurrence := range taskOccurrences {
		taskItems[i] = TaskMission{
			ID:             occurrence.ID,
			Level:          string(occurrence.Level),
			Title:          occurrence.Title,
			Description:    occurrence.Description,
			OccurrenceDate: occurrence.OccurrenceDate,
			IsOptional:     occurrence.IsOptional,
			IsCompleted:    occurrence.IsCompleted,
		}
	}

	workoutProgress := buildProgressForWorkouts(workoutItems)
	taskProgress := buildProgressForTasks(taskItems)

	return &TodayMissionsResponse{
		Date: today,
		Progress: Progress{
			Total:     workoutProgress.Total + taskProgress.Total,
			Completed: workoutProgress.Completed + taskProgress.Completed,
			Pending:   workoutProgress.Pending + taskProgress.Pending,
		},
		Workouts: WorkoutMissions{
			Progress: workoutProgress,
			Items:    workoutItems,
		},
		Tasks: TaskMissions{
			Progress: taskProgress,
			Items:    taskItems,
		},
	}, nil
}

func scheduledWorkoutsForDay(workouts []workout.WorkoutDetail, day time.Time) []WorkoutMission {
	targetDay, ok := workoutDayOfWeek(day.Weekday())
	if !ok {
		return []WorkoutMission{}
	}

	items := make([]WorkoutMission, 0, len(workouts))
	for _, w := range workouts {
		if !w.Active || !hasWorkoutDay(w.DaysOfWeek, targetDay) {
			continue
		}
		items = append(items, WorkoutMission{
			ID:          w.ID,
			Name:        w.Name,
			Description: w.Description,
			IsCompleted: w.DoneToday,
		})
	}
	return items
}

func hasWorkoutDay(days workout.DaySlice, target workout.DayOfWeek) bool {
	for _, day := range days {
		if day == target {
			return true
		}
	}
	return false
}

func workoutDayOfWeek(day time.Weekday) (workout.DayOfWeek, bool) {
	switch day {
	case time.Monday:
		return workout.Monday, true
	case time.Tuesday:
		return workout.Tuesday, true
	case time.Wednesday:
		return workout.Wednesday, true
	case time.Thursday:
		return workout.Thursday, true
	case time.Friday:
		return workout.Friday, true
	default:
		return "", false
	}
}

func buildProgressForWorkouts(items []WorkoutMission) Progress {
	progress := Progress{Total: len(items)}
	for _, item := range items {
		if item.IsCompleted {
			progress.Completed++
		}
	}
	progress.Pending = progress.Total - progress.Completed
	return progress
}

func buildProgressForTasks(items []TaskMission) Progress {
	progress := Progress{Total: len(items)}
	for _, item := range items {
		if item.IsCompleted {
			progress.Completed++
		}
	}
	progress.Pending = progress.Total - progress.Completed
	return progress
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
