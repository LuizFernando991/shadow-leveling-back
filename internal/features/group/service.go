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
	ListGroups(ctx context.Context, userID string) ([]Group, error)
	GetGroupDetail(ctx context.Context, groupID, userID string) (*GroupDetail, error)
	LeaveGroup(ctx context.Context, groupID, userID string) error
	Ranking(ctx context.Context, groupID, userID string) ([]RankingEntry, error)
	Feed(ctx context.Context, groupID, userID, cursor string, limit int) (*entities.CursorPage[FeedItem], error)
	SetCover(ctx context.Context, groupID, userID, contentType string, r io.Reader) (*Group, error)
}

type service struct {
	repo     Repository
	uploader storage.Uploader
	loc      *time.Location
}

func NewService(repo Repository, uploader storage.Uploader) Service {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	return &service{repo: repo, uploader: uploader, loc: loc}
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

func (s *service) ListGroups(ctx context.Context, userID string) ([]Group, error) {
	return s.repo.ListUserGroups(ctx, userID)
}

func (s *service) GetGroupDetail(ctx context.Context, groupID, userID string) (*GroupDetail, error) {
	g, err := s.memberGroup(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}

	from, to := weekBounds(time.Now(), s.loc)
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
	from, to := weekBounds(time.Now(), s.loc)
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

	items, err := s.repo.Feed(ctx, groupID, limit+1, afterTime, afterID)
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
