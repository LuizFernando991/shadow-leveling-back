package task

import "time"

type CreateTaskRequest struct {
	Level            Level          `json:"level"               validate:"required,oneof=hard medium easy no_rank"`
	Title            string         `json:"title"               validate:"required,min=1,max=150"`
	Description      *string        `json:"description"         validate:"omitempty,max=1000"`
	InitialDate      time.Time      `json:"initial_date"        validate:"required"`
	FinalDate        time.Time      `json:"final_date"          validate:"required"`
	RecurrenceType   RecurrenceType `json:"recurrence_type"     validate:"required,oneof=one_time weekly daily monthly custom"`
	CustomDaysOfWeek DaySlice       `json:"custom_days_of_week" validate:"omitempty,dive,oneof=sunday monday tuesday wednesday thursday friday saturday"`
	IsOptional       bool           `json:"is_optional"`
}

type CompleteTaskRequest struct {
	Date *time.Time `json:"date"`
}
