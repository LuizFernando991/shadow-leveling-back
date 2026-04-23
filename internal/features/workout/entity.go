package workout

import (
	"strings"
	"time"
)

type ExerciseType string

const (
	ExerciseTypeRepetition ExerciseType = "repetition"
	ExerciseTypeTime       ExerciseType = "time"
)

type Exercise struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      ExerciseType `json:"type"`
	Unit      string       `json:"unit"`
	CreatedAt time.Time    `json:"created_at"`
}

type DayOfWeek string

const (
	Monday    DayOfWeek = "monday"
	Tuesday   DayOfWeek = "tuesday"
	Wednesday DayOfWeek = "wednesday"
	Thursday  DayOfWeek = "thursday"
	Friday    DayOfWeek = "friday"
)

// DaySlice is a slice of DayOfWeek with helpers for Postgres TEXT[] conversion.
type DaySlice []DayOfWeek

// PgLiteral converts the slice to a Postgres array literal: {monday,tuesday}.
func (d DaySlice) PgLiteral() string {
	if len(d) == 0 {
		return "{}"
	}
	parts := make([]string, len(d))
	for i, v := range d {
		parts[i] = string(v)
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// ParseDaySlice parses a comma-separated string returned by array_to_string.
func ParseDaySlice(s string) DaySlice {
	if s == "" {
		return DaySlice{}
	}
	parts := strings.Split(s, ",")
	result := make(DaySlice, len(parts))
	for i, p := range parts {
		result[i] = DayOfWeek(strings.TrimSpace(p))
	}
	return result
}

type Workout struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	DaysOfWeek  DaySlice  `json:"days_of_week"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type WorkoutDetail struct {
	Workout
	Exercises []WorkoutExercise `json:"exercises"`
}

type WorkoutExercise struct {
	ID         string    `json:"id"`
	WorkoutID  string    `json:"workout_id"`
	ExerciseID string    `json:"exercise_id"`
	Exercise   *Exercise `json:"exercise,omitempty"`
	Sets       int       `json:"sets"`
	RepsMin    *int      `json:"reps_min,omitempty"`
	RepsMax    *int      `json:"reps_max,omitempty"`
	Duration   *int      `json:"duration,omitempty"`
	Note       *string   `json:"note,omitempty"`
	SortOrder  int       `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
}

type SessionStatus string

const (
	StatusComplete   SessionStatus = "complete"
	StatusIncomplete SessionStatus = "incomplete"
	StatusSkipped    SessionStatus = "skipped"
)

type WorkoutSession struct {
	ID        string        `json:"id"`
	WorkoutID string        `json:"workout_id"`
	Date      time.Time     `json:"date"`
	Status    SessionStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type WorkoutSessionDetail struct {
	WorkoutSession
	Sets []ExerciseSet `json:"sets"`
}

type ExerciseSet struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	ExerciseID string    `json:"exercise_id"`
	SetNumber  int       `json:"set_number"`
	Reps       *int      `json:"reps,omitempty"`
	Weight     *float64  `json:"weight,omitempty"`
	Duration   *int      `json:"duration,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type MissedSession struct {
	Date        time.Time `json:"date"`
	WorkoutID   string    `json:"workout_id"`
	WorkoutName string    `json:"workout_name"`
}

type ExerciseProgress struct {
	ExerciseID   string        `json:"exercise_id"`
	ExerciseName string        `json:"exercise_name"`
	ExerciseType ExerciseType  `json:"exercise_type"`
	Sessions     []SessionStat `json:"sessions"`
}

type SessionStat struct {
	Date    time.Time   `json:"date"`
	BestSet ExerciseSet `json:"best_set"`
}
