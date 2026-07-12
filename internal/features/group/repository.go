package group

import (
	"context"
	"database/sql"
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
	ListUserGroups(ctx context.Context, userID string) ([]Group, error)
	AddMember(ctx context.Context, groupID, userID string, role Role) error
	RemoveMember(ctx context.Context, groupID, userID string) error
	IsMember(ctx context.Context, groupID, userID string) (bool, error)
	CountMembers(ctx context.Context, groupID string) (int, error)
	SetCover(ctx context.Context, groupID, coverURL string) error
	WeeklyPoints(ctx context.Context, groupID string, from, to time.Time) ([]RankingEntry, error)
	Feed(ctx context.Context, groupID string, limit int, afterTime *time.Time, afterID *string) ([]FeedItem, error)
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

func (r *postgresRepository) ListUserGroups(ctx context.Context, userID string) ([]Group, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+groupColsG+`
		   FROM groups g
		   JOIN group_members gm ON gm.group_id = g.id
		  WHERE gm.user_id = $1
		  ORDER BY g.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("group: list user groups: %w", err)
	}
	defer rows.Close()

	groups := []Group{}
	for rows.Next() {
		var g Group
		if err := scanGroup(rows, &g); err != nil {
			return nil, fmt.Errorf("group: scan group: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
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
		        COUNT(DISTINCT ws.date) AS points
		   FROM group_members gm
		   JOIN users u ON u.id = gm.user_id
		   LEFT JOIN workouts w ON w.user_id = gm.user_id
		   LEFT JOIN workout_sessions ws
		        ON ws.workout_id = w.id
		       AND ws.status = 'complete'
		       AND ws.date BETWEEN $2 AND $3
		  WHERE gm.group_id = $1
		  GROUP BY gm.user_id, u.nickname, u.email
		  ORDER BY points DESC, name ASC`,
		groupID, from, to)
	if err != nil {
		return nil, fmt.Errorf("group: weekly points: %w", err)
	}
	defer rows.Close()

	entries := []RankingEntry{}
	for rows.Next() {
		var e RankingEntry
		if err := rows.Scan(&e.UserID, &e.Name, &e.Points); err != nil {
			return nil, fmt.Errorf("group: scan ranking: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Feed returns completed sessions of the group's members, newest first, using
// keyset pagination on (created_at, id).
func (r *postgresRepository) Feed(ctx context.Context, groupID string, limit int, afterTime *time.Time, afterID *string) ([]FeedItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ws.id, w.user_id, COALESCE(u.nickname, split_part(u.email, '@', 1)) AS name,
		        w.name AS workout_name, ws.photo_url, ws.created_at
		   FROM workout_sessions ws
		   JOIN workouts w ON w.id = ws.workout_id
		   JOIN users u ON u.id = w.user_id
		   JOIN group_members gm ON gm.user_id = w.user_id AND gm.group_id = $1
		  WHERE ws.status = 'complete'
		    AND ($2::timestamptz IS NULL OR (ws.created_at, ws.id) < ($2, $3))
		  ORDER BY ws.created_at DESC, ws.id DESC
		  LIMIT $4`,
		groupID, afterTime, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("group: feed: %w", err)
	}
	defer rows.Close()

	items := []FeedItem{}
	for rows.Next() {
		var it FeedItem
		if err := rows.Scan(&it.SessionID, &it.UserID, &it.Name, &it.WorkoutName, &it.PhotoURL, &it.CreatedAt); err != nil {
			return nil, fmt.Errorf("group: scan feed: %w", err)
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
