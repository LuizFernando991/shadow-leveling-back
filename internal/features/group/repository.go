package group

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// isUniqueViolation reports whether err is a Postgres unique-constraint error
// (SQLSTATE 23505), used to detect the rare invite-code collision.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

type Repository interface {
	CreateGroup(ctx context.Context, name, inviteCode, ownerID string) (*Group, error)
	GetGroup(ctx context.Context, id string) (*Group, error)
	GroupByInviteCode(ctx context.Context, code string) (*Group, error)
	ListUserGroups(ctx context.Context, userID string) ([]GroupListItem, error)
	AddMember(ctx context.Context, groupID, userID string, role Role) error
	RemoveMember(ctx context.Context, groupID, userID string) error
	IsMember(ctx context.Context, groupID, userID string) (bool, error)
	CountMembers(ctx context.Context, groupID string) (int, error)
	SetCover(ctx context.Context, groupID, coverURL string) error
	WeeklyPoints(ctx context.Context, groupID string, from, to time.Time) ([]RankingEntry, error)
	Feed(ctx context.Context, groupID, userID string, limit int, afterTime *time.Time, afterID *string) ([]FeedItem, error)

	// SessionInGroup loads a completed session's post header, but only if its
	// author belongs to the group. Returns sql.ErrNoRows otherwise.
	SessionInGroup(ctx context.Context, sessionID, groupID string) (*SessionDetail, error)
	ReactionSummary(ctx context.Context, sessionID, groupID string) ([]ReactionCount, error)
	MyReaction(ctx context.Context, sessionID, groupID, userID string) (*string, error)
	SetReaction(ctx context.Context, sessionID, groupID, userID, emoji string) error
	DeleteReaction(ctx context.Context, sessionID, groupID, userID string) error
	CountComments(ctx context.Context, sessionID, groupID string) (int, error)
	ListComments(ctx context.Context, sessionID, groupID, userID string, limit int, afterTime *time.Time, afterID *string) ([]CommentItem, error)
	AddComment(ctx context.Context, sessionID, groupID, userID, body string) (*CommentItem, error)
	// DeleteComment removes the comment only when it belongs to userID within the
	// group. Returns false if nothing matched (missing or not the author).
	DeleteComment(ctx context.Context, commentID, groupID, userID string) (bool, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &postgresRepository{db: db}
}

const groupCols = `id, name, cover_url, invite_code, owner_id, created_at`
const groupColsG = `g.id, g.name, g.cover_url, g.invite_code, g.owner_id, g.created_at`

func scanGroup(s interface{ Scan(...any) error }, g *Group) error {
	return s.Scan(&g.ID, &g.Name, &g.CoverURL, &g.InviteCode, &g.OwnerID, &g.CreatedAt)
}

func (r *postgresRepository) CreateGroup(ctx context.Context, name, inviteCode, ownerID string) (*Group, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("group: begin tx: %w", err)
	}
	defer tx.Rollback()

	var g Group
	if err := scanGroup(tx.QueryRowContext(ctx,
		`INSERT INTO groups (name, invite_code, owner_id) VALUES ($1, $2, $3) RETURNING `+groupCols,
		name, inviteCode, ownerID,
	), &g); err != nil {
		return nil, fmt.Errorf("group: insert group: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, $3)`,
		g.ID, ownerID, RoleOwner,
	); err != nil {
		return nil, fmt.Errorf("group: insert owner membership: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("group: commit: %w", err)
	}
	return &g, nil
}

func (r *postgresRepository) GetGroup(ctx context.Context, id string) (*Group, error) {
	var g Group
	if err := scanGroup(r.db.QueryRowContext(ctx,
		`SELECT `+groupCols+` FROM groups WHERE id = $1`, id), &g); err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *postgresRepository) GroupByInviteCode(ctx context.Context, code string) (*Group, error) {
	var g Group
	if err := scanGroup(r.db.QueryRowContext(ctx,
		`SELECT `+groupCols+` FROM groups WHERE invite_code = $1`, code), &g); err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *postgresRepository) ListUserGroups(ctx context.Context, userID string) ([]GroupListItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+groupColsG+`,
		        (SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id) AS member_count,
		        COALESCE((SELECT json_agg(a.avatar_url) FROM (
		            SELECT u.avatar_url FROM group_members m2
		            JOIN users u ON u.id = m2.user_id
		            WHERE m2.group_id = g.id AND u.avatar_url IS NOT NULL
		            ORDER BY m2.joined_at LIMIT 3) a), '[]') AS member_avatars
		   FROM groups g
		   JOIN group_members gm ON gm.group_id = g.id
		  WHERE gm.user_id = $1
		  ORDER BY g.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("group: list user groups: %w", err)
	}
	defer rows.Close()

	items := []GroupListItem{}
	for rows.Next() {
		var it GroupListItem
		var avatarsJSON []byte
		if err := rows.Scan(&it.ID, &it.Name, &it.CoverURL, &it.InviteCode, &it.OwnerID, &it.CreatedAt,
			&it.MemberCount, &avatarsJSON); err != nil {
			return nil, fmt.Errorf("group: scan group: %w", err)
		}
		if err := json.Unmarshal(avatarsJSON, &it.MemberAvatars); err != nil {
			return nil, fmt.Errorf("group: unmarshal avatars: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *postgresRepository) AddMember(ctx context.Context, groupID, userID string, role Role) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role) VALUES ($1, $2, $3)`,
		groupID, userID, role)
	if err != nil {
		return fmt.Errorf("group: add member: %w", err)
	}
	return nil
}

func (r *postgresRepository) RemoveMember(ctx context.Context, groupID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID)
	if err != nil {
		return fmt.Errorf("group: remove member: %w", err)
	}
	return nil
}

func (r *postgresRepository) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id = $1 AND user_id = $2)`,
		groupID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("group: is member: %w", err)
	}
	return exists, nil
}

func (r *postgresRepository) CountMembers(ctx context.Context, groupID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM group_members WHERE group_id = $1`, groupID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("group: count members: %w", err)
	}
	return n, nil
}

func (r *postgresRepository) SetCover(ctx context.Context, groupID, coverURL string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE groups SET cover_url = $2 WHERE id = $1`, groupID, coverURL)
	if err != nil {
		return fmt.Errorf("group: set cover: %w", err)
	}
	return nil
}

// WeeklyPoints returns every member's points for the [from, to] date window,
// where a point is one distinct day with a completed workout session.
// LEFT JOIN keeps members with zero points in the result.
func (r *postgresRepository) WeeklyPoints(ctx context.Context, groupID string, from, to time.Time) ([]RankingEntry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT gm.user_id,
		        COALESCE(u.nickname, split_part(u.email, '@', 1)) AS name,
		        u.avatar_url,
		        COUNT(DISTINCT ws.date) AS points
		   FROM group_members gm
		   JOIN users u ON u.id = gm.user_id
		   LEFT JOIN workouts w ON w.user_id = gm.user_id
		   LEFT JOIN workout_sessions ws
		        ON ws.workout_id = w.id
		       AND ws.status = 'complete'
		       AND ws.date BETWEEN $2 AND $3
		  WHERE gm.group_id = $1
		  GROUP BY gm.user_id, u.nickname, u.email, u.avatar_url
		  ORDER BY points DESC, name ASC`,
		groupID, from, to)
	if err != nil {
		return nil, fmt.Errorf("group: weekly points: %w", err)
	}
	defer rows.Close()

	entries := []RankingEntry{}
	for rows.Next() {
		var e RankingEntry
		if err := rows.Scan(&e.UserID, &e.Name, &e.AvatarURL, &e.Points); err != nil {
			return nil, fmt.Errorf("group: scan ranking: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Feed returns completed sessions of the group's members, newest first, using
// keyset pagination on (created_at, id). Each item carries this group's social
// counts and the viewer's own reaction.
func (r *postgresRepository) Feed(ctx context.Context, groupID, userID string, limit int, afterTime *time.Time, afterID *string) ([]FeedItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ws.id, w.user_id, COALESCE(u.nickname, split_part(u.email, '@', 1)) AS name,
		        u.avatar_url, w.name AS workout_name, ws.photo_url, ws.created_at,
		        (SELECT COUNT(*) FROM session_reactions sr WHERE sr.session_id = ws.id AND sr.group_id = $1) AS reaction_count,
		        (SELECT COUNT(*) FROM session_comments  sc WHERE sc.session_id = ws.id AND sc.group_id = $1) AS comment_count,
		        (SELECT sr.emoji FROM session_reactions sr WHERE sr.session_id = ws.id AND sr.group_id = $1 AND sr.user_id = $2) AS my_reaction,
		        (SELECT sr.emoji FROM session_reactions sr WHERE sr.session_id = ws.id AND sr.group_id = $1
		           GROUP BY sr.emoji ORDER BY COUNT(*) DESC, MIN(sr.created_at) ASC LIMIT 1) AS top_emoji
		   FROM workout_sessions ws
		   JOIN workouts w ON w.id = ws.workout_id
		   JOIN users u ON u.id = w.user_id
		   JOIN group_members gm ON gm.user_id = w.user_id AND gm.group_id = $1
		  WHERE ws.status = 'complete'
		    AND ($3::timestamptz IS NULL OR (ws.created_at, ws.id) < ($3, $4))
		  ORDER BY ws.created_at DESC, ws.id DESC
		  LIMIT $5`,
		groupID, userID, afterTime, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("group: feed: %w", err)
	}
	defer rows.Close()

	items := []FeedItem{}
	for rows.Next() {
		var it FeedItem
		if err := rows.Scan(&it.SessionID, &it.UserID, &it.Name, &it.AvatarURL, &it.WorkoutName, &it.PhotoURL, &it.CreatedAt,
			&it.ReactionCount, &it.CommentCount, &it.MyReaction, &it.TopEmoji); err != nil {
			return nil, fmt.Errorf("group: scan feed: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// SessionInGroup returns the post header for a session whose author is a member
// of the group. Returns sql.ErrNoRows if the session is not completed or its
// author is not in the group.
func (r *postgresRepository) SessionInGroup(ctx context.Context, sessionID, groupID string) (*SessionDetail, error) {
	var d SessionDetail
	err := r.db.QueryRowContext(ctx,
		`SELECT ws.id, w.user_id, COALESCE(u.nickname, split_part(u.email, '@', 1)) AS name,
		        u.avatar_url, w.name AS workout_name, ws.photo_url, ws.created_at
		   FROM workout_sessions ws
		   JOIN workouts w ON w.id = ws.workout_id
		   JOIN users u ON u.id = w.user_id
		   JOIN group_members gm ON gm.user_id = w.user_id AND gm.group_id = $2
		  WHERE ws.id = $1 AND ws.status = 'complete'`,
		sessionID, groupID).Scan(&d.SessionID, &d.UserID, &d.Name, &d.AvatarURL, &d.WorkoutName, &d.PhotoURL, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *postgresRepository) ReactionSummary(ctx context.Context, sessionID, groupID string) ([]ReactionCount, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT emoji, COUNT(*) AS n
		   FROM session_reactions
		  WHERE session_id = $1 AND group_id = $2
		  GROUP BY emoji
		  ORDER BY n DESC, emoji ASC`,
		sessionID, groupID)
	if err != nil {
		return nil, fmt.Errorf("group: reaction summary: %w", err)
	}
	defer rows.Close()

	out := []ReactionCount{}
	for rows.Next() {
		var rc ReactionCount
		if err := rows.Scan(&rc.Emoji, &rc.Count); err != nil {
			return nil, fmt.Errorf("group: scan reaction: %w", err)
		}
		out = append(out, rc)
	}
	return out, rows.Err()
}

func (r *postgresRepository) MyReaction(ctx context.Context, sessionID, groupID, userID string) (*string, error) {
	var emoji *string
	err := r.db.QueryRowContext(ctx,
		`SELECT emoji FROM session_reactions WHERE session_id = $1 AND group_id = $2 AND user_id = $3`,
		sessionID, groupID, userID).Scan(&emoji)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("group: my reaction: %w", err)
	}
	return emoji, nil
}

func (r *postgresRepository) SetReaction(ctx context.Context, sessionID, groupID, userID, emoji string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO session_reactions (session_id, group_id, user_id, emoji)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (session_id, group_id, user_id)
		     DO UPDATE SET emoji = EXCLUDED.emoji, created_at = NOW()`,
		sessionID, groupID, userID, emoji)
	if err != nil {
		return fmt.Errorf("group: set reaction: %w", err)
	}
	return nil
}

func (r *postgresRepository) DeleteReaction(ctx context.Context, sessionID, groupID, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM session_reactions WHERE session_id = $1 AND group_id = $2 AND user_id = $3`,
		sessionID, groupID, userID)
	if err != nil {
		return fmt.Errorf("group: delete reaction: %w", err)
	}
	return nil
}

func (r *postgresRepository) CountComments(ctx context.Context, sessionID, groupID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM session_comments WHERE session_id = $1 AND group_id = $2`,
		sessionID, groupID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("group: count comments: %w", err)
	}
	return n, nil
}

// ListComments returns a session's comments newest-first, keyset-paginated on
// (created_at, id). is_mine marks the viewer's own comments.
func (r *postgresRepository) ListComments(ctx context.Context, sessionID, groupID, userID string, limit int, afterTime *time.Time, afterID *string) ([]CommentItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT c.id, c.user_id, COALESCE(u.nickname, split_part(u.email, '@', 1)) AS name,
		        u.avatar_url, c.body, c.created_at, (c.user_id = $3) AS is_mine
		   FROM session_comments c
		   JOIN users u ON u.id = c.user_id
		  WHERE c.session_id = $1 AND c.group_id = $2
		    AND ($4::timestamptz IS NULL OR (c.created_at, c.id) < ($4, $5))
		  ORDER BY c.created_at DESC, c.id DESC
		  LIMIT $6`,
		sessionID, groupID, userID, afterTime, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("group: list comments: %w", err)
	}
	defer rows.Close()

	items := []CommentItem{}
	for rows.Next() {
		var it CommentItem
		if err := rows.Scan(&it.ID, &it.UserID, &it.Name, &it.AvatarURL, &it.Body, &it.CreatedAt, &it.IsMine); err != nil {
			return nil, fmt.Errorf("group: scan comment: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func (r *postgresRepository) AddComment(ctx context.Context, sessionID, groupID, userID, body string) (*CommentItem, error) {
	var it CommentItem
	err := r.db.QueryRowContext(ctx,
		`WITH ins AS (
		     INSERT INTO session_comments (session_id, group_id, user_id, body)
		     VALUES ($1, $2, $3, $4)
		     RETURNING id, user_id, body, created_at
		 )
		 SELECT ins.id, ins.user_id, COALESCE(u.nickname, split_part(u.email, '@', 1)) AS name,
		        u.avatar_url, ins.body, ins.created_at
		   FROM ins JOIN users u ON u.id = ins.user_id`,
		sessionID, groupID, userID, body).Scan(&it.ID, &it.UserID, &it.Name, &it.AvatarURL, &it.Body, &it.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("group: add comment: %w", err)
	}
	it.IsMine = true
	return &it, nil
}

func (r *postgresRepository) DeleteComment(ctx context.Context, commentID, groupID, userID string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM session_comments WHERE id = $1 AND group_id = $2 AND user_id = $3`,
		commentID, groupID, userID)
	if err != nil {
		return false, fmt.Errorf("group: delete comment: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("group: delete comment rows: %w", err)
	}
	return n > 0, nil
}
