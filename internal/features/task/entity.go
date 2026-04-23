package task

import (
	"strings"
	"time"
)

type Level string

const (
	LevelHard   Level = "hard"
	LevelMedium Level = "medium"
	LevelEasy   Level = "easy"
	LevelNoRank Level = "no_rank"
)

type RecurrenceType string

const (
	RecurrenceOneTime RecurrenceType = "one_time"
	RecurrenceWeekly  RecurrenceType = "weekly"
	RecurrenceDaily   RecurrenceType = "daily"
	RecurrenceMonthly RecurrenceType = "monthly"
	RecurrenceCustom  RecurrenceType = "custom"
)

type DayOfWeek string

const (
	Sunday    DayOfWeek = "sunday"
	Monday    DayOfWeek = "monday"
	Tuesday   DayOfWeek = "tuesday"
	Wednesday DayOfWeek = "wednesday"
	Thursday  DayOfWeek = "thursday"
	Friday    DayOfWeek = "friday"
	Saturday  DayOfWeek = "saturday"
)

type DaySlice []DayOfWeek

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

type Task struct {
	ID               string         `json:"id"`
	UserID           string         `json:"user_id"`
	Level            Level          `json:"level"`
	Title            string         `json:"title"`
	Description      *string        `json:"description,omitempty"`
	InitialDate      time.Time      `json:"initial_date"`
	FinalDate        time.Time      `json:"final_date"`
	RecurrenceType   RecurrenceType `json:"recurrence_type"`
	CustomDaysOfWeek DaySlice       `json:"custom_days_of_week"`
	IsOptional       bool           `json:"is_optional"`
	IsCompleted      bool           `json:"is_completed"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type TaskOccurrence struct {
	Task
	OccurrenceDate time.Time `json:"occurrence_date"`
}
