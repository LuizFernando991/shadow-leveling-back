package workout

import "time"

type CreateExerciseRequest struct {
	Name string       `json:"name" validate:"required,min=1,max=100"`
	Type ExerciseType `json:"type" validate:"required,oneof=repetition time"`
	Unit string       `json:"unit" validate:"required,min=1,max=20"`
}

type CreateWorkoutRequest struct {
	Name        string   `json:"name"         validate:"required,min=1,max=100"`
	Description *string  `json:"description"  validate:"omitempty,max=500"`
	DaysOfWeek  DaySlice `json:"days_of_week" validate:"required,min=1,dive,oneof=monday tuesday wednesday thursday friday"`
}

type UpdateWorkoutRequest struct {
	Name        *string  `json:"name"         validate:"omitempty,min=1,max=100"`
	Description *string  `json:"description"  validate:"omitempty,max=500"`
	DaysOfWeek  DaySlice `json:"days_of_week" validate:"omitempty,min=1,dive,oneof=monday tuesday wednesday thursday friday"`
	Active      *bool    `json:"active"`
}

type AddWorkoutExerciseRequest struct {
	ExerciseID string  `json:"exercise_id" validate:"required,uuid"`
	Sets       int     `json:"sets"        validate:"required,min=1"`
	RepsMin    *int    `json:"reps_min"    validate:"omitempty,min=1"`
	RepsMax    *int    `json:"reps_max"    validate:"omitempty,min=1"`
	Duration   *int    `json:"duration"    validate:"omitempty,min=1"`
	Note       *string `json:"note"        validate:"omitempty,max=500"`
	SortOrder  int     `json:"sort_order"  validate:"min=0"`
}

type UpdateWorkoutExerciseRequest struct {
	Sets      *int    `json:"sets"       validate:"omitempty,min=1"`
	RepsMin   *int    `json:"reps_min"   validate:"omitempty,min=1"`
	RepsMax   *int    `json:"reps_max"   validate:"omitempty,min=1"`
	Duration  *int    `json:"duration"   validate:"omitempty,min=1"`
	Note      *string `json:"note"       validate:"omitempty,max=500"`
	SortOrder *int    `json:"sort_order" validate:"omitempty,min=0"`
}

type ReorderWorkoutExercisesRequest struct {
	Exercises []ReorderWorkoutExerciseItem `json:"exercises" validate:"required,min=1,max=50,dive"`
}

type ReorderWorkoutExerciseItem struct {
	ID        string `json:"id"         validate:"required,uuid"`
	SortOrder int    `json:"sort_order" validate:"min=0"`
}

type CreateWorkoutSessionRequest struct {
	WorkoutID string        `json:"workout_id" validate:"required,uuid"`
	Date      time.Time     `json:"date"       validate:"required"`
	Status    SessionStatus `json:"status"     validate:"required,oneof=complete incomplete skipped"`
}

type UpdateWorkoutSessionRequest struct {
	Status *SessionStatus `json:"status" validate:"omitempty,oneof=complete incomplete skipped"`
}

type RecordSetRequest struct {
	ExerciseID string   `json:"exercise_id" validate:"required,uuid"`
	SetNumber  int      `json:"set_number"  validate:"required,min=1"`
	Reps       *int     `json:"reps"        validate:"omitempty,min=0"`
	Weight     *float64 `json:"weight"      validate:"omitempty,min=0"`
	Duration   *int     `json:"duration"    validate:"omitempty,min=0"`
}

type UpdateSetRequest struct {
	Reps     *int     `json:"reps"     validate:"omitempty,min=0"`
	Weight   *float64 `json:"weight"   validate:"omitempty,min=0"`
	Duration *int     `json:"duration" validate:"omitempty,min=0"`
}
