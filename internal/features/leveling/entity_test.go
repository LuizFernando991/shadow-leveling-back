package leveling

import "testing"

func TestLevelForXP(t *testing.T) {
	cases := []struct {
		xp   int
		want int
	}{
		{0, 1},
		{-1, 1},
		{99, 1},
		{100, 2},
		{399, 2},
		{400, 3},
		{900, 4},
		{1600, 5},
		{2500, 6},
	}
	for _, c := range cases {
		got := LevelForXP(c.xp)
		if got != c.want {
			t.Errorf("LevelForXP(%d) = %d, want %d", c.xp, got, c.want)
		}
	}
}

func TestXPForLevel(t *testing.T) {
	cases := []struct {
		level int
		want  int
	}{
		{1, 0},
		{2, 100},
		{3, 400},
		{4, 900},
		{5, 1600},
		{10, 8100},
	}
	for _, c := range cases {
		got := XPForLevel(c.level)
		if got != c.want {
			t.Errorf("XPForLevel(%d) = %d, want %d", c.level, got, c.want)
		}
	}
}

func TestRankForLevel(t *testing.T) {
	cases := []struct {
		level int
		want  string
	}{
		{1, "E-Rank"},
		{4, "E-Rank"},
		{5, "D-Rank"},
		{9, "D-Rank"},
		{10, "C-Rank"},
		{19, "C-Rank"},
		{20, "B-Rank"},
		{29, "B-Rank"},
		{30, "A-Rank"},
		{49, "A-Rank"},
		{50, "S-Rank"},
		{99, "S-Rank"},
	}
	for _, c := range cases {
		got := RankForLevel(c.level)
		if got != c.want {
			t.Errorf("RankForLevel(%d) = %q, want %q", c.level, got, c.want)
		}
	}
}

func TestStreakBonus(t *testing.T) {
	cases := []struct {
		streak int
		want   int
	}{
		{0, 0},
		{1, 5},
		{3, 15},
		{9, 45},
		{10, 50},
		{20, 50},
	}
	for _, c := range cases {
		got := StreakBonus(c.streak)
		if got != c.want {
			t.Errorf("StreakBonus(%d) = %d, want %d", c.streak, got, c.want)
		}
	}
}

func TestBuildLevelInfo(t *testing.T) {
	// Level 2 starts at 100 XP, level 3 at 400 — width is 300.
	// User has 250 XP → into=150, pct=50.
	xp := UserXP{TotalXP: 250, CurrentStreak: 3}
	info := BuildLevelInfo(xp)

	if info.Level != 2 {
		t.Errorf("Level = %d, want 2", info.Level)
	}
	if info.Rank != "E-Rank" {
		t.Errorf("Rank = %q, want E-Rank", info.Rank)
	}
	if info.XPIntoLevel != 150 {
		t.Errorf("XPIntoLevel = %d, want 150", info.XPIntoLevel)
	}
	if info.XPForNextLevel != 300 {
		t.Errorf("XPForNextLevel = %d, want 300", info.XPForNextLevel)
	}
	if info.ProgressPct != 50 {
		t.Errorf("ProgressPct = %d, want 50", info.ProgressPct)
	}
	if info.CurrentStreak != 3 {
		t.Errorf("CurrentStreak = %d, want 3", info.CurrentStreak)
	}

	// Zero XP → level 1, E-Rank, all zeros.
	zero := BuildLevelInfo(UserXP{})
	if zero.Level != 1 || zero.TotalXP != 0 || zero.ProgressPct != 0 {
		t.Errorf("zero-XP info incorrect: %+v", zero)
	}
}