package task

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Repository interface {
	CreateTask(ctx context.Context, userID string, req CreateTaskRequest) (*Task, error)
	GetTask(ctx context.Context, id string) (*Task, error)
	ListTasksForRange(ctx context.Context, userID string, from, to time.Time) ([]Task, error)
	CompleteTask(ctx context.Context, taskID string, date time.Time) error
	ListCompletionDates(ctx context.Context, userID string, taskIDs []string, from, to time.Time) (map[string]map[string]bool, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

const taskCols = `id, user_id, level, title, description, initial_date, final_date, recurrence_type, array_to_string(custom_days_of_week, ','), is_optional, false, created_at, updated_at`

func scanTask(s interface{ Scan(...any) error }, t *Task) error {
	var daysStr string
	if err := s.Scan(
		&t.ID,
		&t.UserID,
		&t.Level,
		&t.Title,
		&t.Description,
		&t.InitialDate,
		&t.FinalDate,
		&t.RecurrenceType,
		&daysStr,
		&t.IsOptional,
		&t.IsCompleted,
		&t.CreatedAt,
		&t.UpdatedAt,
	); err != nil {
		return err
	}
	t.CustomDaysOfWeek = ParseDaySlice(daysStr)
	return nil
}

func (r *postgresRepository) CreateTask(ctx context.Context, userID string, req CreateTaskRequest) (*Task, error) {
	var t Task
	err := scanTask(r.db.QueryRowContext(ctx,
		`INSERT INTO tasks (user_id, level, title, description, initial_date, final_date, recurrence_type, custom_days_of_week, is_optional)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::text[], $9)
		 RETURNING `+taskCols,
		userID,
		req.Level,
		req.Title,
		req.Description,
		req.InitialDate,
		req.FinalDate,
		req.RecurrenceType,
		req.CustomDaysOfWeek.PgLiteral(),
		req.IsOptional,
	), &t)
	if err != nil {
		return nil, fmt.Errorf("task: create task: %w", err)
	}
	return &t, nil
}

func (r *postgresRepository) GetTask(ctx context.Context, id string) (*Task, error) {
	var t Task
	err := scanTask(r.db.QueryRowContext(ctx,
		`SELECT `+taskCols+` FROM tasks WHERE id = $1`,
		id,
	), &t)
	if err != nil {
		return nil, fmt.Errorf("task: get task: %w", err)
	}
	return &t, nil
}

func (r *postgresRepository) ListTasksForRange(ctx context.Context, userID string, from, to time.Time) ([]Task, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+taskCols+`
		 FROM tasks
		 WHERE user_id = $1
		   AND initial_date <= $3
		   AND final_date >= $2
		 ORDER BY initial_date, created_at`,
		userID,
		from,
		to,
	)
	if err != nil {
		return nil, fmt.Errorf("task: list tasks for range: %w", err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := scanTask(rows, &t); err != nil {
			return nil, fmt.Errorf("task: scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task: rows error: %w", err)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return tasks, nil
}

func (r *postgresRepository) CompleteTask(ctx context.Context, taskID string, date time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO task_completions (task_id, completed_date)
		 VALUES ($1, $2)
		 ON CONFLICT (task_id, completed_date) DO NOTHING`,
		taskID,
		date,
	)
	if err != nil {
		return fmt.Errorf("task: complete task: %w", err)
	}
	return nil
}

func (r *postgresRepository) ListCompletionDates(ctx context.Context, userID string, taskIDs []string, from, to time.Time) (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool)
	if len(taskIDs) == 0 {
		return result, nil
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT tc.task_id, tc.completed_date
		 FROM task_completions tc
		 JOIN tasks t ON t.id = tc.task_id
		 WHERE t.user_id = $1
		   AND tc.task_id = ANY($2::uuid[])
		   AND tc.completed_date BETWEEN $3 AND $4`,
		userID,
		"{"+joinStrings(taskIDs)+"}",
		from,
		to,
	)
	if err != nil {
		return nil, fmt.Errorf("task: list completion dates: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var taskID string
		var completedDate time.Time
		if err := rows.Scan(&taskID, &completedDate); err != nil {
			return nil, fmt.Errorf("task: scan completion date: %w", err)
		}
		if result[taskID] == nil {
			result[taskID] = make(map[string]bool)
		}
		result[taskID][dateKey(completedDate)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("task: completion rows error: %w", err)
	}
	return result, nil
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return ""
	}
	result := values[0]
	for _, v := range values[1:] {
		result += "," + v
	}
	return result
}
