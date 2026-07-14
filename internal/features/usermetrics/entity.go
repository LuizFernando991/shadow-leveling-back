package usermetrics

import "time"

type Progress struct {
	Total     int `json:"total"`
	Completed int `json:"completed"`
	Pending   int `json:"pending"`
}

type WorkoutMission struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Description         *string `json:"description,omitempty"`
	IsCompleted         bool    `json:"is_completed"`
	ExerciseCount       int     `json:"exercise_count"`
	EstimatedDurationMin int    `json:"estimated_duration_min"`
}

type TaskMission struct {
	ID             string    `json:"id"`
	Level          string    `json:"level"`
	Title          string    `json:"title"`
	Description    *string   `json:"description,omitempty"`
	OccurrenceDate time.Time `json:"occurrence_date"`
	IsOptional     bool      `json:"is_optional"`
	IsCompleted    bool      `json:"is_completed"`
}

type WorkoutMissions struct {
	Progress Progress         `json:"progress"`
	Items    []WorkoutMission `json:"items"`
}

type TaskMissions struct {
	Progress Progress      `json:"progress"`
	Items    []TaskMission `json:"items"`
}

type TodayMissionsResponse struct {
	Date     time.Time       `json:"date"`
	Progress Progress        `json:"progress"`
	Workouts WorkoutMissions `json:"workouts"`
	Tasks    TaskMissions    `json:"tasks"`
}

type WeeklyGoal struct {
	Completed int `json:"completed"`
	Scheduled int `json:"scheduled"`
}

type WeeklySummaryResponse struct {
	Goal WeeklyGoal `json:"goal"`
}
