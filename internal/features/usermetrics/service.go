package usermetrics

import (
	"context"
	"time"

	"github.com/LuizFernando991/gym-api/internal/features/task"
	"github.com/LuizFernando991/gym-api/internal/features/workout"
)

type WorkoutService interface {
	ListWorkouts(ctx context.Context, userID string) ([]workout.WorkoutDetail, error)
	CountCompletedSessions(ctx context.Context, userID string, from, to time.Time) (int, error)
}

type TaskService interface {
	ListByDay(ctx context.Context, userID string, date time.Time) ([]task.TaskOccurrence, error)
}

// GoalReader reads the user's weekly-goal setting (days/week). nil goal means
// the user hasn't defined a goal yet → scheduled=0 (frontend prompts to set).
type GoalReader interface {
	GetWeeklyGoalDays(ctx context.Context, userID string) (*int, error)
}

type Service interface {
	GetTodayMissions(ctx context.Context, userID string) (*TodayMissionsResponse, error)
	GetWeeklySummary(ctx context.Context, userID string) (*WeeklySummaryResponse, error)
}

type service struct {
	workoutSvc WorkoutService
	taskSvc    TaskService
	goalReader GoalReader
	now        func() time.Time
}

func NewService(workoutSvc WorkoutService, taskSvc TaskService, goalReader GoalReader) Service {
	return &service{
		workoutSvc: workoutSvc,
		taskSvc:    taskSvc,
		goalReader: goalReader,
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
	targetDay := workoutDayOfWeek(day.Weekday())

	items := make([]WorkoutMission, 0, len(workouts))
	for _, w := range workouts {
		if !w.Active || !hasWorkoutDay(w.DaysOfWeek, targetDay) {
			continue
		}
		items = append(items, WorkoutMission{
			ID:                   w.ID,
			Name:                 w.Name,
			Description:          w.Description,
			IsCompleted:          w.DoneToday,
			ExerciseCount:        len(w.Exercises),
			EstimatedDurationMin: estimateDurationMin(w.Exercises),
		})
	}
	return items
}

// estimateDurationMin computes an estimated workout length in minutes.
// Per-set: 1 min execution + 1 min rest. Time-typed exercises use their
// duration (in seconds) per set.
// ponytail: no transition time between exercises; refine if estimates drift.
func estimateDurationMin(exercises []workout.WorkoutExercise) int {
	totalSeconds := 0
	for _, ex := range exercises {
		if ex.Exercise != nil && ex.Exercise.Type == workout.ExerciseTypeTime && ex.Duration != nil {
			totalSeconds += *ex.Duration * ex.Sets
			continue
		}
		totalSeconds += ex.Sets * 120 // 2 min per set (1 exec + 1 rest)
	}
	return (totalSeconds + 30) / 60 // round to nearest minute
}

func hasWorkoutDay(days workout.DaySlice, target workout.DayOfWeek) bool {
	for _, day := range days {
		if day == target {
			return true
		}
	}
	return false
}

func workoutDayOfWeek(day time.Weekday) workout.DayOfWeek {
	switch day {
	case time.Monday:
		return workout.Monday
	case time.Tuesday:
		return workout.Tuesday
	case time.Wednesday:
		return workout.Wednesday
	case time.Thursday:
		return workout.Thursday
	case time.Friday:
		return workout.Friday
	case time.Saturday:
		return workout.Saturday
	default:
		return workout.Sunday
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

func (s *service) GetWeeklySummary(ctx context.Context, userID string) (*WeeklySummaryResponse, error) {
	now := dateOnly(s.now())
	from, to := currentWeekRange(now)

	// Weekly goal is a user-set number of workouts/week. nil = not defined;
	// scheduled=0 signals the frontend to prompt the user to define a goal.
	scheduled := 0
	if s.goalReader != nil {
		days, err := s.goalReader.GetWeeklyGoalDays(ctx, userID)
		if err != nil {
			return nil, err
		}
		if days != nil {
			scheduled = *days
		}
	}

	completed, err := s.workoutSvc.CountCompletedSessions(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}

	return &WeeklySummaryResponse{
		Goal: WeeklyGoal{Completed: completed, Scheduled: scheduled},
	}, nil
}

// currentWeekRange returns the Monday..Sunday range containing `now`.
func currentWeekRange(now time.Time) (time.Time, time.Time) {
	daysSinceMonday := int(now.Weekday()) - int(time.Monday)
	if daysSinceMonday < 0 {
		daysSinceMonday += 7
	}
	from := now.AddDate(0, 0, -daysSinceMonday)
	to := from.AddDate(0, 0, 6)
	return from, to
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
