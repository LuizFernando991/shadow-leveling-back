package task

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrForbidden      = errors.New("forbidden")
	ErrInvalidDate    = errors.New("invalid date")
	ErrNotScheduled   = errors.New("task is not scheduled for date")
	ErrInvalidRequest = errors.New("invalid request")
)

type Service interface {
	CreateTask(ctx context.Context, userID string, req CreateTaskRequest) (*Task, error)
	CompleteTask(ctx context.Context, id, userID string, req CompleteTaskRequest) (*TaskOccurrence, error)
	ListUncompletedByDay(ctx context.Context, userID string, date time.Time) ([]TaskOccurrence, error)
	ListByDay(ctx context.Context, userID string, date time.Time) ([]TaskOccurrence, error)
	ListByMonth(ctx context.Context, userID string, year int, month time.Month) ([]TaskOccurrence, error)
}

type service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) Service {
	return &service{repo: repo, now: time.Now}
}

func (s *service) CreateTask(ctx context.Context, userID string, req CreateTaskRequest) (*Task, error) {
	req.InitialDate = dateOnly(req.InitialDate)
	req.FinalDate = dateOnly(req.FinalDate)

	if req.FinalDate.Before(req.InitialDate) {
		return nil, ErrInvalidDate
	}
	if req.RecurrenceType == RecurrenceCustom && len(req.CustomDaysOfWeek) == 0 {
		return nil, ErrInvalidRequest
	}
	if req.RecurrenceType != RecurrenceCustom {
		req.CustomDaysOfWeek = DaySlice{}
	}

	t, err := s.repo.CreateTask(ctx, userID, req)
	if err != nil {
		return nil, fmt.Errorf("task: create task: %w", err)
	}
	return t, nil
}

func (s *service) CompleteTask(ctx context.Context, id, userID string, req CompleteTaskRequest) (*TaskOccurrence, error) {
	t, err := s.repo.GetTask(ctx, id)
	if isNotFound(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("task: get task: %w", err)
	}
	if t.UserID != userID {
		return nil, ErrForbidden
	}

	completedDate := dateOnly(s.now())
	if req.Date != nil {
		completedDate = dateOnly(*req.Date)
	}
	if !occursOn(*t, completedDate) {
		return nil, ErrNotScheduled
	}

	if err := s.repo.CompleteTask(ctx, id, completedDate); err != nil {
		return nil, fmt.Errorf("task: complete task: %w", err)
	}
	t.IsCompleted = true
	return &TaskOccurrence{Task: *t, OccurrenceDate: completedDate}, nil
}

func (s *service) ListUncompletedByDay(ctx context.Context, userID string, date time.Time) ([]TaskOccurrence, error) {
	occurrences, err := s.ListByDay(ctx, userID, date)
	if err != nil {
		return nil, err
	}
	uncompleted := make([]TaskOccurrence, 0, len(occurrences))
	for _, occurrence := range occurrences {
		if !occurrence.IsCompleted {
			uncompleted = append(uncompleted, occurrence)
		}
	}
	return uncompleted, nil
}

func (s *service) ListByDay(ctx context.Context, userID string, date time.Time) ([]TaskOccurrence, error) {
	d := dateOnly(date)
	return s.listOccurrences(ctx, userID, d, d)
}

func (s *service) ListByMonth(ctx context.Context, userID string, year int, month time.Month) ([]TaskOccurrence, error) {
	if year < 1 || month < time.January || month > time.December {
		return nil, ErrInvalidDate
	}
	from := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, -1)
	return s.listOccurrences(ctx, userID, from, to)
}

func (s *service) listOccurrences(ctx context.Context, userID string, from, to time.Time) ([]TaskOccurrence, error) {
	tasks, err := s.repo.ListTasksForRange(ctx, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("task: list tasks for range: %w", err)
	}

	taskIDs := make([]string, 0, len(tasks))
	for _, t := range tasks {
		taskIDs = append(taskIDs, t.ID)
	}
	completions, err := s.repo.ListCompletionDates(ctx, userID, taskIDs, from, to)
	if err != nil {
		return nil, fmt.Errorf("task: list completion dates: %w", err)
	}

	var occurrences []TaskOccurrence
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		for _, t := range tasks {
			if !occursOn(t, d) {
				continue
			}
			occurrence := TaskOccurrence{Task: t, OccurrenceDate: d}
			occurrence.IsCompleted = completions[t.ID][dateKey(d)]
			occurrences = append(occurrences, occurrence)
		}
	}
	if occurrences == nil {
		occurrences = []TaskOccurrence{}
	}
	return occurrences, nil
}

func occursOn(t Task, d time.Time) bool {
	d = dateOnly(d)
	if d.Before(dateOnly(t.InitialDate)) || d.After(dateOnly(t.FinalDate)) {
		return false
	}

	switch t.RecurrenceType {
	case RecurrenceOneTime:
		return sameDate(d, t.InitialDate)
	case RecurrenceDaily:
		return true
	case RecurrenceWeekly:
		return d.Weekday() == t.InitialDate.Weekday()
	case RecurrenceMonthly:
		return d.Day() == t.InitialDate.Day()
	case RecurrenceCustom:
		return containsDay(t.CustomDaysOfWeek, dayOfWeek(d.Weekday()))
	default:
		return false
	}
}

func containsDay(days DaySlice, day DayOfWeek) bool {
	for _, d := range days {
		if d == day {
			return true
		}
	}
	return false
}

func dayOfWeek(w time.Weekday) DayOfWeek {
	switch w {
	case time.Sunday:
		return Sunday
	case time.Monday:
		return Monday
	case time.Tuesday:
		return Tuesday
	case time.Wednesday:
		return Wednesday
	case time.Thursday:
		return Thursday
	case time.Friday:
		return Friday
	default:
		return Saturday
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func sameDate(a, b time.Time) bool {
	return dateKey(a) == dateKey(b)
}

func dateKey(t time.Time) string {
	return dateOnly(t).Format("2006-01-02")
}
