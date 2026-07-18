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

// GroupListItem is a group as shown in the user's groups list: identity plus a
// member count and a few member avatar URLs for the avatar stack.
type GroupListItem struct {
	Group
	MemberCount   int      `json:"member_count"`
	MemberAvatars []string `json:"member_avatars"` // up to 3, non-null URLs
}

// GroupDetail is the group page header: identity plus the week's headline scores.
type GroupDetail struct {
	Group
	TopScore     int     `json:"top_score"`      // points of the 1st place this week
	TopName      string  `json:"top_name"`       // name of the 1st place this week
	TopAvatarURL *string `json:"top_avatar_url"` // avatar of the 1st place this week
	MyScore      int     `json:"my_score"`       // logged-in user's points this week
	MemberCnt    int     `json:"member_count"`
}

// RankingEntry is one member's weekly standing.
type RankingEntry struct {
	UserID    string  `json:"user_id"`
	Name      string  `json:"name"`
	AvatarURL *string `json:"avatar_url"`
	Points    int     `json:"points"`
}

// FeedItem is one completed workout shown in the group feed.
type FeedItem struct {
	SessionID     string    `json:"session_id"`
	UserID        string    `json:"user_id"`
	Name          string    `json:"name"`
	AvatarURL     *string   `json:"avatar_url"` // the author's profile photo
	WorkoutName   string    `json:"workout_name"`
	PhotoURL      *string   `json:"photo_url"` // the session's progress photo, not the author's avatar
	CreatedAt     time.Time `json:"created_at"`
	ReactionCount int       `json:"reaction_count"`
	CommentCount  int       `json:"comment_count"`
	MyReaction    *string   `json:"my_reaction"` // this viewer's emoji on this session, or null
	TopEmoji      *string   `json:"top_emoji"`   // most-used emoji on this session (tie: earliest), or null
}

// ReactionCount is one emoji and how many members used it on a session.
type ReactionCount struct {
	Emoji string `json:"emoji"`
	Count int    `json:"count"`
}

// SessionDetail is a completed workout viewed inside a group: the post header
// plus its social state. Only sessions whose author is a member of the group
// are visible here.
type SessionDetail struct {
	SessionID    string          `json:"session_id"`
	UserID       string          `json:"user_id"`
	Name         string          `json:"name"`
	AvatarURL    *string         `json:"avatar_url"`
	WorkoutName  string          `json:"workout_name"`
	PhotoURL     *string         `json:"photo_url"`
	CreatedAt    time.Time       `json:"created_at"`
	Reactions    []ReactionCount `json:"reactions"`
	MyReaction   *string         `json:"my_reaction"`
	CommentCount int             `json:"comment_count"`
}

// CommentItem is one comment on a session, with its author's display fields.
type CommentItem struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatar_url"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	IsMine    bool      `json:"is_mine"`
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
