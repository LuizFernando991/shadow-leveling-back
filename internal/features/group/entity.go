package group

import (
	"crypto/rand"
	"time"
)

type Role string

const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

// Group is a social group users join to compete on a weekly ranking.
type Group struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CoverURL   *string   `json:"cover_url"`
	InviteCode string    `json:"invite_code"`
	OwnerID    string    `json:"owner_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// GroupDetail is the group page header: identity plus the week's headline scores.
type GroupDetail struct {
	Group
	TopScore  int `json:"top_score"` // points of the 1st place this week
	MyScore   int `json:"my_score"`  // logged-in user's points this week
	MemberCnt int `json:"member_count"`
}

// RankingEntry is one member's weekly standing.
type RankingEntry struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
	Points int    `json:"points"`
}

// FeedItem is one completed workout shown in the group feed.
type FeedItem struct {
	SessionID   string    `json:"session_id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	WorkoutName string    `json:"workout_name"`
	PhotoURL    *string   `json:"photo_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// weekBounds returns the Monday and Sunday (date-only, inclusive) of the week
// containing now, evaluated in loc. Pure — unit-tested in entity_test.go.
func weekBounds(now time.Time, loc *time.Location) (start, end time.Time) {
	n := now.In(loc)
	// Go: Sunday=0..Saturday=6. Shift so Monday=0..Sunday=6.
	daysFromMonday := (int(n.Weekday()) + 6) % 7
	start = time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -daysFromMonday)
	end = start.AddDate(0, 0, 6)
	return start, end
}

const inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no ambiguous 0/O/1/I

// newInviteCode returns a short, human-shareable, unambiguous code.
func newInviteCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = inviteAlphabet[int(b[i])%len(inviteAlphabet)]
	}
	return string(b)
}
