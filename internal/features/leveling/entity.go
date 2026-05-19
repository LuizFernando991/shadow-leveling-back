package leveling

import "math"

// XP rules. See PRD.md §4.
const (
	// WorkoutCompletionXP is the base XP granted for completing a workout session.
	WorkoutCompletionXP = 50
	// StreakBonusPerDay is the XP added per consecutive day in the streak.
	StreakBonusPerDay = 5
	// MaxStreakBonus caps the streak bonus regardless of streak length.
	MaxStreakBonus = 50
)

// XP event reasons (also persisted in xp_events.reason).
const (
	ReasonWorkoutCompleted = "workout_session_completed"
	ReasonStreakBonus      = "streak_bonus"
)

// SourceTypeWorkoutSession identifies a workout session as the XP source.
const SourceTypeWorkoutSession = "workout_session"

// UserXP is the persisted progression state for a user.
type UserXP struct {
	TotalXP       int
	CurrentStreak int
}

// LevelInfo is the derived progression view returned by the API.
type LevelInfo struct {
	Level          int    `json:"level"`
	Rank           string `json:"rank"`
	TotalXP        int    `json:"total_xp"`
	XPIntoLevel    int    `json:"xp_into_level"`
	XPForNextLevel int    `json:"xp_for_next_level"`
	ProgressPct    int    `json:"progress_pct"`
	CurrentStreak  int    `json:"current_streak"`
}

// XPForLevel returns the cumulative XP required to reach level n.
// Formula (PRD §4.4): 100 * (n-1)^2. Level 1 starts at 0 XP.
func XPForLevel(n int) int {
	if n <= 1 {
		return 0
	}
	d := n - 1
	return 100 * d * d
}

// LevelForXP returns the level for a given total XP: the greatest n such
// that XPForLevel(n) <= totalXP. Always >= 1.
func LevelForXP(totalXP int) int {
	if totalXP <= 0 {
		return 1
	}
	// n-1 <= sqrt(totalXP/100)  =>  n = floor(sqrt(totalXP/100)) + 1
	n := int(math.Floor(math.Sqrt(float64(totalXP)/100.0))) + 1
	if n < 1 {
		return 1
	}
	return n
}

// RankForLevel maps a level to its themed rank (PRD §4.4).
func RankForLevel(level int) string {
	switch {
	case level >= 50:
		return "S-Rank"
	case level >= 30:
		return "A-Rank"
	case level >= 20:
		return "B-Rank"
	case level >= 10:
		return "C-Rank"
	case level >= 5:
		return "D-Rank"
	default:
		return "E-Rank"
	}
}

// StreakBonus computes the bonus XP for a resulting streak (PRD §4.3).
func StreakBonus(streak int) int {
	if streak <= 0 {
		return 0
	}
	bonus := streak * StreakBonusPerDay
	if bonus > MaxStreakBonus {
		return MaxStreakBonus
	}
	return bonus
}

// BuildLevelInfo derives the full LevelInfo from persisted XP state.
func BuildLevelInfo(xp UserXP) LevelInfo {
	level := LevelForXP(xp.TotalXP)

	floor := XPForLevel(level)
	next := XPForLevel(level + 1)
	width := next - floor
	into := xp.TotalXP - floor

	pct := 0
	if width > 0 {
		pct = int(math.Floor(float64(into) / float64(width) * 100))
	}

	return LevelInfo{
		Level:          level,
		Rank:           RankForLevel(level),
		TotalXP:        xp.TotalXP,
		XPIntoLevel:    into,
		XPForNextLevel: width,
		ProgressPct:    pct,
		CurrentStreak:  xp.CurrentStreak,
	}
}