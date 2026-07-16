package auth_test

import (
	"net/http"
	"testing"
)

// TestGroupSocial exercises reactions and comments on a group feed session:
// posting, toggling, authorization, feed counters and delete rules.
func TestGroupSocial(t *testing.T) {
	truncate(t)
	monday, _ := weekDates(t)

	ownerTok := registerUser(t, "social-owner@example.com", "password123")
	memberTok := registerUser(t, "social-member@example.com", "password123")

	// Owner creates a group; member joins.
	resp := request(t, http.MethodPost, "/groups", map[string]string{"name": "Hunters"}, ownerTok)
	assertStatus(t, resp, http.StatusCreated)
	var grp struct {
		ID         string `json:"id"`
		InviteCode string `json:"invite_code"`
	}
	decodeBody(t, resp, &grp)
	resp = request(t, http.MethodPost, "/groups/join", map[string]string{"invite_code": grp.InviteCode}, memberTok)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Owner logs a completed session (appears in the group feed).
	sessionID := createCompletedSession(t, ownerTok, monday)
	base := "/groups/" + grp.ID + "/sessions/" + sessionID

	// Member reacts 🔥 -> detail shows the aggregate and my_reaction.
	resp = request(t, http.MethodPut, base+"/reaction", map[string]string{"emoji": "🔥"}, memberTok)
	assertStatus(t, resp, http.StatusOK)
	var detail struct {
		Reactions []struct {
			Emoji string `json:"emoji"`
			Count int    `json:"count"`
		} `json:"reactions"`
		MyReaction   *string `json:"my_reaction"`
		CommentCount int     `json:"comment_count"`
	}
	decodeBody(t, resp, &detail)
	if len(detail.Reactions) != 1 || detail.Reactions[0].Emoji != "🔥" || detail.Reactions[0].Count != 1 {
		t.Fatalf("reactions = %+v, want one 🔥 x1", detail.Reactions)
	}
	if detail.MyReaction == nil || *detail.MyReaction != "🔥" {
		t.Fatalf("my_reaction = %v, want 🔥", detail.MyReaction)
	}

	// Switching to 💪 replaces (still one reaction, now 💪).
	resp = request(t, http.MethodPut, base+"/reaction", map[string]string{"emoji": "💪"}, memberTok)
	assertStatus(t, resp, http.StatusOK)
	decodeBody(t, resp, &detail)
	if len(detail.Reactions) != 1 || detail.Reactions[0].Emoji != "💪" {
		t.Fatalf("after switch reactions = %+v, want one 💪", detail.Reactions)
	}

	// Re-sending 💪 toggles it off -> no reactions, my_reaction null.
	resp = request(t, http.MethodPut, base+"/reaction", map[string]string{"emoji": "💪"}, memberTok)
	assertStatus(t, resp, http.StatusOK)
	decodeBody(t, resp, &detail)
	if len(detail.Reactions) != 0 || detail.MyReaction != nil {
		t.Fatalf("after toggle-off reactions=%+v my=%v, want empty/null", detail.Reactions, detail.MyReaction)
	}

	// Member comments; author (owner) comments too.
	resp = request(t, http.MethodPost, base+"/comments", map[string]string{"body": "Monstro! 🔥"}, memberTok)
	assertStatus(t, resp, http.StatusCreated)
	var memberComment struct {
		ID     string `json:"id"`
		IsMine bool   `json:"is_mine"`
	}
	decodeBody(t, resp, &memberComment)
	if !memberComment.IsMine {
		t.Fatal("member's own comment should have is_mine=true")
	}
	resp = request(t, http.MethodPost, base+"/comments", map[string]string{"body": "valeu!"}, ownerTok)
	assertStatus(t, resp, http.StatusCreated)
	resp.Body.Close()

	// List comments (newest-first): 2 items, is_mine reflects the viewer.
	resp = request(t, http.MethodGet, base+"/comments", nil, memberTok)
	assertStatus(t, resp, http.StatusOK)
	var page struct {
		Data []struct {
			ID     string `json:"id"`
			Body   string `json:"body"`
			IsMine bool   `json:"is_mine"`
		} `json:"data"`
	}
	decodeBody(t, resp, &page)
	if len(page.Data) != 2 {
		t.Fatalf("comments = %d, want 2", len(page.Data))
	}

	// Empty body -> 400.
	resp = request(t, http.MethodPost, base+"/comments", map[string]string{"body": ""}, memberTok)
	assertStatus(t, resp, http.StatusBadRequest)
	resp.Body.Close()

	// Member cannot delete the owner's comment (must be 403 or their own only).
	// Find the owner's comment id from the list.
	var ownerCommentID string
	for _, c := range page.Data {
		if !c.IsMine {
			ownerCommentID = c.ID
		}
	}
	if ownerCommentID == "" {
		t.Fatal("expected to find the owner's comment")
	}
	resp = request(t, http.MethodDelete, base+"/comments/"+ownerCommentID, nil, memberTok)
	assertStatus(t, resp, http.StatusForbidden)
	resp.Body.Close()

	// Member deletes their own comment -> 204.
	resp = request(t, http.MethodDelete, base+"/comments/"+memberComment.ID, nil, memberTok)
	assertStatus(t, resp, http.StatusNoContent)
	resp.Body.Close()

	// A non-member cannot view or interact.
	outsiderTok := registerUser(t, "social-outsider@example.com", "password123")
	resp = request(t, http.MethodGet, base, nil, outsiderTok)
	assertStatus(t, resp, http.StatusForbidden)
	resp.Body.Close()

	// Feed reflects social counters for the viewer: 1 comment left, no reaction.
	resp = request(t, http.MethodGet, "/groups/"+grp.ID+"/feed", nil, memberTok)
	assertStatus(t, resp, http.StatusOK)
	var feed struct {
		Data []struct {
			SessionID     string  `json:"session_id"`
			ReactionCount int     `json:"reaction_count"`
			CommentCount  int     `json:"comment_count"`
			MyReaction    *string `json:"my_reaction"`
			TopEmoji      *string `json:"top_emoji"`
		} `json:"data"`
	}
	decodeBody(t, resp, &feed)
	if len(feed.Data) != 1 {
		t.Fatalf("feed items = %d, want 1", len(feed.Data))
	}
	got := feed.Data[0]
	if got.ReactionCount != 0 || got.CommentCount != 1 || got.MyReaction != nil {
		t.Fatalf("feed social = %+v, want reactions 0, comments 1, my_reaction nil", got)
	}
	if got.TopEmoji != nil {
		t.Fatalf("top_emoji = %v, want nil (no reactions left)", got.TopEmoji)
	}

	// Member reacts 💪 again -> feed's top_emoji reflects the actual emoji, not 🔥.
	resp = request(t, http.MethodPut, base+"/reaction", map[string]string{"emoji": "💪"}, memberTok)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()
	resp = request(t, http.MethodGet, "/groups/"+grp.ID+"/feed", nil, memberTok)
	assertStatus(t, resp, http.StatusOK)
	decodeBody(t, resp, &feed)
	if len(feed.Data) != 1 || feed.Data[0].TopEmoji == nil || *feed.Data[0].TopEmoji != "💪" {
		t.Fatalf("top_emoji = %v, want 💪", feed.Data[0].TopEmoji)
	}
}
