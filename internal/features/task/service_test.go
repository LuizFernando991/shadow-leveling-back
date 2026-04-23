package task

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

type fakeRepository struct {
	tasks       map[string]Task
	completions map[string]map[string]bool
}

func newFakeRepository(tasks ...Task) *fakeRepository {
	repo := &fakeRepository{
		tasks:       make(map[string]Task),
		completions: make(map[string]map[string]bool),
	}
	for _, t := range tasks {
		repo.tasks[t.ID] = t
	}
	return repo
}

func (r *fakeRepository) CreateTask(ctx context.Context, userID string, req CreateTaskRequest) (*Task, error) {
	t := Task{
		ID:               "created-task",
		UserID:           userID,
		Level:            req.Level,
		Title:            req.Title,
		Description:      req.Description,
		InitialDate:      req.InitialDate,
		FinalDate:        req.FinalDate,
		RecurrenceType:   req.RecurrenceType,
		CustomDaysOfWeek: req.CustomDaysOfWeek,
		IsOptional:       req.IsOptional,
	}
	r.tasks[t.ID] = t
	return &t, nil
}

func (r *fakeRepository) GetTask(ctx context.Context, id string) (*Task, error) {
	t, ok := r.tasks[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return &t, nil
}

func (r *fakeRepository) ListTasksForRange(ctx context.Context, userID string, from, to time.Time) ([]Task, error) {
	var tasks []Task
	for _, t := range r.tasks {
		if t.UserID == userID && !t.InitialDate.After(to) && !t.FinalDate.Before(from) {
			tasks = append(tasks, t)
		}
	}
	return tasks, nil
}

func (r *fakeRepository) CompleteTask(ctx context.Context, taskID string, date time.Time) error {
	if r.completions[taskID] == nil {
		r.completions[taskID] = make(map[string]bool)
	}
	r.completions[taskID][dateKey(date)] = true
	return nil
}

func (r *fakeRepository) ListCompletionDates(ctx context.Context, userID string, taskIDs []string, from, to time.Time) (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool)
	for _, taskID := range taskIDs {
		for key, completed := range r.completions[taskID] {
			d, err := time.Parse("2006-01-02", key)
			if err != nil || d.Before(from) || d.After(to) || !completed {
				continue
			}
			if result[taskID] == nil {
				result[taskID] = make(map[string]bool)
			}
			result[taskID][key] = true
		}
	}
	return result, nil
}

func TestListUncompletedByDayHonorsPerDateCompletion(t *testing.T) {
	initial := mustDate(t, "2026-04-01")
	final := mustDate(t, "2026-04-30")
	repo := newFakeRepository(Task{
		ID:             "daily-task",
		UserID:         "user-1",
		Level:          LevelMedium,
		Title:          "Daily task",
		InitialDate:    initial,
		FinalDate:      final,
		RecurrenceType: RecurrenceDaily,
	})
	svc := NewService(repo)

	completeDate := mustDate(t, "2026-04-10")
	if _, err := svc.CompleteTask(context.Background(), "daily-task", "user-1", CompleteTaskRequest{Date: &completeDate}); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	uncompleted, err := svc.ListUncompletedByDay(context.Background(), "user-1", completeDate)
	if err != nil {
		t.Fatalf("list uncompleted for completed date: %v", err)
	}
	if len(uncompleted) != 0 {
		t.Fatalf("expected no uncompleted tasks for completed date, got %d", len(uncompleted))
	}

	nextDate := mustDate(t, "2026-04-11")
	uncompleted, err = svc.ListUncompletedByDay(context.Background(), "user-1", nextDate)
	if err != nil {
		t.Fatalf("list uncompleted for next date: %v", err)
	}
	if len(uncompleted) != 1 {
		t.Fatalf("expected task to remain uncompleted on a different date, got %d", len(uncompleted))
	}
}

func TestCustomRecurrenceOnlyOccursOnSelectedWeekdays(t *testing.T) {
	repo := newFakeRepository(Task{
		ID:               "custom-task",
		UserID:           "user-1",
		Level:            LevelEasy,
		Title:            "Custom task",
		InitialDate:      mustDate(t, "2026-04-01"),
		FinalDate:        mustDate(t, "2026-04-30"),
		RecurrenceType:   RecurrenceCustom,
		CustomDaysOfWeek: DaySlice{Monday, Wednesday},
	})
	svc := NewService(repo)

	wednesday := mustDate(t, "2026-04-01")
	occurrences, err := svc.ListByDay(context.Background(), "user-1", wednesday)
	if err != nil {
		t.Fatalf("list by day: %v", err)
	}
	if len(occurrences) != 1 {
		t.Fatalf("expected occurrence on Wednesday, got %d", len(occurrences))
	}

	thursday := mustDate(t, "2026-04-02")
	occurrences, err = svc.ListByDay(context.Background(), "user-1", thursday)
	if err != nil {
		t.Fatalf("list by day: %v", err)
	}
	if len(occurrences) != 0 {
		t.Fatalf("expected no occurrence on Thursday, got %d", len(occurrences))
	}
}

func TestCompleteTaskRejectsUnscheduledDate(t *testing.T) {
	repo := newFakeRepository(Task{
		ID:             "weekly-task",
		UserID:         "user-1",
		Level:          LevelHard,
		Title:          "Weekly task",
		InitialDate:    mustDate(t, "2026-04-06"),
		FinalDate:      mustDate(t, "2026-04-30"),
		RecurrenceType: RecurrenceWeekly,
	})
	svc := NewService(repo)

	tuesday := mustDate(t, "2026-04-07")
	if _, err := svc.CompleteTask(context.Background(), "weekly-task", "user-1", CompleteTaskRequest{Date: &tuesday}); err != ErrNotScheduled {
		t.Fatalf("expected ErrNotScheduled, got %v", err)
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
