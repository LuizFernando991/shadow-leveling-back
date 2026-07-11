package group

import (
	"testing"
	"time"
)

func TestWeekBounds(t *testing.T) {
	loc := time.UTC
	// 2026-07-08 is a Wednesday.
	cases := []struct {
		name      string
		now       time.Time
		wantStart string // Monday
		wantEnd   string // Sunday
	}{
		{"wednesday", time.Date(2026, 7, 8, 15, 30, 0, 0, loc), "2026-07-06", "2026-07-12"},
		{"monday start of day", time.Date(2026, 7, 6, 0, 0, 0, 0, loc), "2026-07-06", "2026-07-12"},
		{"sunday end of week", time.Date(2026, 7, 12, 23, 59, 59, 0, loc), "2026-07-06", "2026-07-12"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start, end := weekBounds(c.now, loc)
			if got := start.Format("2006-01-02"); got != c.wantStart {
				t.Errorf("start = %s, want %s", got, c.wantStart)
			}
			if got := end.Format("2006-01-02"); got != c.wantEnd {
				t.Errorf("end = %s, want %s", got, c.wantEnd)
			}
			if start.Hour() != 0 || start.Minute() != 0 {
				t.Errorf("start not truncated to midnight: %v", start)
			}
		})
	}
}

func TestNewInviteCode(t *testing.T) {
	code := newInviteCode()
	if len(code) != 6 {
		t.Fatalf("invite code length = %d, want 6", len(code))
	}
	for _, r := range code {
		if !containsRune(inviteAlphabet, r) {
			t.Errorf("invite code contains ambiguous/invalid rune %q", r)
		}
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}
