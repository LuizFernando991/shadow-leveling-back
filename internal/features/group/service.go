package group

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/LuizFernando991/gym-api/internal/infra/storage"
	"github.com/LuizFernando991/gym-api/internal/shared/apptime"
	"github.com/LuizFernando991/gym-api/internal/shared/entities"
)

var (
	ErrNotFound         = errors.New("group not found")
	ErrForbidden        = errors.New("access denied")
	ErrInvalidInvite    = errors.New("invalid invite code")
	ErrAlreadyMember    = errors.New("already a member")
	ErrInvalidCursor    = errors.New("invalid cursor")
	ErrUnsupportedImage = errors.New("unsupported image type")
)

const (
	defaultFeedSize = 20
	maxFeedSize     = 100
	inviteRetries   = 5
)

type Service interface {
	CreateGroup(ctx context.Context, ownerID string, req CreateGroupRequest) (*Group, error)
	JoinGroup(ctx context.Context, userID string, req JoinGroupRequest) (*Group, error)
	ListGroups(ctx context.Context, userID string) ([]GroupListItem, error)
	GetGroupDetail(ctx context.Context, groupID, userID string) (*GroupDetail, error)
	LeaveGroup(ctx context.Context, groupID, userID string) error
	Ranking(ctx context.Context, groupID, userID string) ([]RankingEntry, error)
	Feed(ctx context.Context, groupID, userID, cursor string, limit int) (*entities.CursorPage[FeedItem], error)
	SetCover(ctx context.Context, groupID, userID, contentType string, r io.Reader) (*Group, error)

	SessionDetail(ctx context.Context, groupID, sessionID, userID string) (*SessionDetail, error)
	SetReaction(ctx context.Context, groupID, sessionID, userID, emoji string) (*SessionDetail, error)
	RemoveReaction(ctx context.Context, groupID, sessionID, userID string) (*SessionDetail, error)
	Comments(ctx context.Context, groupID, sessionID, userID, cursor string, limit int) (*entities.CursorPage[CommentItem], error)
	AddComment(ctx context.Context, groupID, sessionID, userID string, req AddCommentRequest) (*CommentItem, error)
	DeleteComment(ctx context.Context, groupID, sessionID, commentID, userID string) error
}

// Notifier fires best-effort push notifications to a session's author. Satisfied
// by the notification module; nil in tests. Mirrors workout.GroupNotifier.
type Notifier interface {
	NotifySessionReaction(ctx context.Context, actorID, sessionID string)
	NotifySessionComment(ctx context.Context, actorID, sessionID string)
}

type service struct {
	repo     Repository
	uploader storage.Uploader
	notifier Notifier
}

func NewService(repo Repository, uploader storage.Uploader, notifier Notifier) Service {
	return &service{repo: repo, uploader: uploader, notifier: notifier}
}

func (s *service) CreateGroup(ctx context.Context, ownerID string, req CreateGroupRequest) (*Group, error) {
	// Retry on the rare invite-code collision (unique constraint).
	for i := 0; i < inviteRetries; i++ {
		g, err := s.repo.CreateGroup(ctx, req.Name, newInviteCode(), ownerID)
		if err == nil {
			return g, nil
		}
		if isUniqueViolation(err) {
			continue
		}
		return nil, err
	}
	return nil, fmt.Errorf("group: exhausted invite code retries")
}

func (s *service) JoinGroup(ctx context.Context, userID string, req JoinGroupRequest) (*Group, error) {
	g, err := s.repo.GroupByInviteCode(ctx, req.InviteCode)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidInvite
	}
	if err != nil {
		return nil, fmt.Errorf("group: lookup invite: %w", err)
	}

	member, err := s.repo.IsMember(ctx, g.ID, userID)
	if err != nil {
		return nil, err
	}
	if member {
		return nil, ErrAlreadyMember
	}
	if err := s.repo.AddMember(ctx, g.ID, userID, RoleMember); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *service) ListGroups(ctx context.Context, userID string) ([]GroupListItem, error) {
	return s.repo.ListUserGroups(ctx, userID)
}

func (s *service) GetGroupDetail(ctx context.Context, groupID, userID string) (*GroupDetail, error) {
	g, err := s.memberGroup(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}

	from, to := weekBounds(time.Now(), apptime.Location)
	ranking, err := s.repo.WeeklyPoints(ctx, groupID, from, to)
	if err != nil {
		return nil, err
	}
	count, err := s.repo.CountMembers(ctx, groupID)
	if err != nil {
		return nil, err
	}

	detail := &GroupDetail{Group: *g, MemberCnt: count}
	if len(ranking) > 0 {
		detail.TopScore = ranking[0].Points // ordered by points desc
		detail.TopName = ranking[0].Name
		detail.TopAvatarURL = ranking[0].AvatarURL
	}
	for _, e := range ranking {
		if e.UserID == userID {
			detail.MyScore = e.Points
			break
		}
	}
	return detail, nil
}

func (s *service) LeaveGroup(ctx context.Context, groupID, userID string) error {
	if _, err := s.memberGroup(ctx, groupID, userID); err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, groupID, userID)
}

func (s *service) Ranking(ctx context.Context, groupID, userID string) ([]RankingEntry, error) {
	if _, err := s.memberGroup(ctx, groupID, userID); err != nil {
		return nil, err
	}
	from, to := weekBounds(time.Now(), apptime.Location)
	return s.repo.WeeklyPoints(ctx, groupID, from, to)
}

func (s *service) Feed(ctx context.Context, groupID, userID, cursor string, limit int) (*entities.CursorPage[FeedItem], error) {
	if _, err := s.memberGroup(ctx, groupID, userID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > maxFeedSize {
		limit = defaultFeedSize
	}

	var afterTime *time.Time
	var afterID *string
	if cursor != "" {
		t, id, err := decodeFeedCursor(cursor)
		if err != nil {
			return nil, err
		}
		afterTime, afterID = &t, &id
	}

	items, err := s.repo.Feed(ctx, groupID, userID, limit+1, afterTime, afterID)
	if err != nil {
		return nil, err
	}

	page := &entities.CursorPage[FeedItem]{Data: items}
	if len(items) > limit {
		page.Data = items[:limit]
		last := page.Data[len(page.Data)-1]
		next := encodeFeedCursor(last.CreatedAt, last.SessionID)
		page.Cursor = entities.CursorMeta{NextCursor: &next, HasMore: true}
	}
	return page, nil
}

// SessionDetail returns a group member's completed workout as a social post:
// header + reaction summary + the viewer's own reaction + comment count.
func (s *service) SessionDetail(ctx context.Context, groupID, sessionID, userID string) (*SessionDetail, error) {
	if _, err := s.memberGroup(ctx, groupID, userID); err != nil {
		return nil, err
	}
	return s.buildSessionDetail(ctx, groupID, sessionID, userID)
}

// SetReaction sets the viewer's emoji on a session (one per user). Re-sending
// the same emoji removes it (toggle). Notifies the author when a reaction lands.
func (s *service) SetReaction(ctx context.Context, groupID, sessionID, userID, emoji string) (*SessionDetail, error) {
	if _, err := s.memberGroup(ctx, groupID, userID); err != nil {
		return nil, err
	}
	// Authorize the session (author must be a member) before writing.
	if _, err := s.repo.SessionInGroup(ctx, sessionID, groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	current, err := s.repo.MyReaction(ctx, sessionID, groupID, userID)
	if err != nil {
		return nil, err
	}
	if current != nil && *current == emoji {
		if err := s.repo.DeleteReaction(ctx, sessionID, groupID, userID); err != nil {
			return nil, err
		}
	} else {
		if err := s.repo.SetReaction(ctx, sessionID, groupID, userID, emoji); err != nil {
			return nil, err
		}
		if s.notifier != nil {
			go s.notifier.NotifySessionReaction(context.Background(), userID, sessionID)
		}
	}
	return s.buildSessionDetail(ctx, groupID, sessionID, userID)
}

// RemoveReaction clears the viewer's reaction on a session, if any.
func (s *service) RemoveReaction(ctx context.Context, groupID, sessionID, userID string) (*SessionDetail, error) {
	if _, err := s.memberGroup(ctx, groupID, userID); err != nil {
		return nil, err
	}
	if _, err := s.repo.SessionInGroup(ctx, sessionID, groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := s.repo.DeleteReaction(ctx, sessionID, groupID, userID); err != nil {
		return nil, err
	}
	return s.buildSessionDetail(ctx, groupID, sessionID, userID)
}

// buildSessionDetail assembles a session's post header with its social state.
func (s *service) buildSessionDetail(ctx context.Context, groupID, sessionID, userID string) (*SessionDetail, error) {
	d, err := s.repo.SessionInGroup(ctx, sessionID, groupID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if d.Reactions, err = s.repo.ReactionSummary(ctx, sessionID, groupID); err != nil {
		return nil, err
	}
	if d.MyReaction, err = s.repo.MyReaction(ctx, sessionID, groupID, userID); err != nil {
		return nil, err
	}
	if d.CommentCount, err = s.repo.CountComments(ctx, sessionID, groupID); err != nil {
		return nil, err
	}
	return d, nil
}

// Comments returns a session's comments, newest-first, keyset-paginated.
func (s *service) Comments(ctx context.Context, groupID, sessionID, userID, cursor string, limit int) (*entities.CursorPage[CommentItem], error) {
	if _, err := s.memberGroup(ctx, groupID, userID); err != nil {
		return nil, err
	}
	if _, err := s.repo.SessionInGroup(ctx, sessionID, groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if limit <= 0 || limit > maxFeedSize {
		limit = defaultFeedSize
	}

	var afterTime *time.Time
	var afterID *string
	if cursor != "" {
		t, id, err := decodeFeedCursor(cursor)
		if err != nil {
			return nil, err
		}
		afterTime, afterID = &t, &id
	}

	items, err := s.repo.ListComments(ctx, sessionID, groupID, userID, limit+1, afterTime, afterID)
	if err != nil {
		return nil, err
	}

	page := &entities.CursorPage[CommentItem]{Data: items}
	if len(items) > limit {
		page.Data = items[:limit]
		last := page.Data[len(page.Data)-1]
		next := encodeFeedCursor(last.CreatedAt, last.ID)
		page.Cursor = entities.CursorMeta{NextCursor: &next, HasMore: true}
	}
	return page, nil
}

// AddComment posts a comment on a session and notifies its author.
func (s *service) AddComment(ctx context.Context, groupID, sessionID, userID string, req AddCommentRequest) (*CommentItem, error) {
	if _, err := s.memberGroup(ctx, groupID, userID); err != nil {
		return nil, err
	}
	if _, err := s.repo.SessionInGroup(ctx, sessionID, groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	c, err := s.repo.AddComment(ctx, sessionID, groupID, userID, req.Body)
	if err != nil {
		return nil, err
	}
	if s.notifier != nil {
		go s.notifier.NotifySessionComment(context.Background(), userID, sessionID)
	}
	return c, nil
}

// DeleteComment removes the caller's own comment; ErrForbidden if it isn't theirs.
func (s *service) DeleteComment(ctx context.Context, groupID, sessionID, commentID, userID string) error {
	if _, err := s.memberGroup(ctx, groupID, userID); err != nil {
		return err
	}
	ok, err := s.repo.DeleteComment(ctx, commentID, groupID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func (s *service) SetCover(ctx context.Context, groupID, userID, contentType string, r io.Reader) (*Group, error) {
	g, err := s.repo.GetGroup(ctx, groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if g.OwnerID != userID {
		return nil, ErrForbidden
	}
	ext, ok := storage.ExtForContentType(contentType)
	if !ok {
		return nil, ErrUnsupportedImage
	}

	url, err := s.uploader.Upload(ctx, "group-covers/"+groupID+ext, contentType, r)
	if err != nil {
		return nil, fmt.Errorf("group: upload cover: %w", err)
	}
	if err := s.repo.SetCover(ctx, groupID, url); err != nil {
		return nil, err
	}
	g.CoverURL = &url
	return g, nil
}

// memberGroup loads the group and asserts the user belongs to it.
func (s *service) memberGroup(ctx context.Context, groupID, userID string) (*Group, error) {
	g, err := s.repo.GetGroup(ctx, groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	member, err := s.repo.IsMember(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, ErrForbidden
	}
	return g, nil
}

// ── feed cursor ──────────────────────────────────────────────────────────────

type feedCursor struct {
	T time.Time `json:"t"`
	I string    `json:"i"`
}

func encodeFeedCursor(t time.Time, id string) string {
	b, _ := json.Marshal(feedCursor{T: t, I: id})
	return base64.StdEncoding.EncodeToString(b)
}

func decodeFeedCursor(cursor string) (time.Time, string, error) {
	b, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", ErrInvalidCursor
	}
	var c feedCursor
	if err := json.Unmarshal(b, &c); err != nil {
		return time.Time{}, "", ErrInvalidCursor
	}
	return c.T, c.I, nil
}
