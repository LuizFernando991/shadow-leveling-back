package auth_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"
	"time"
)

// uploadImage posts a tiny fake image to path (multipart "image" field) and
// returns the response. The test suite uses a noop uploader, so bytes are discarded.
func uploadImage(t *testing.T, method, path, token string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", `form-data; name="image"; filename="photo.jpg"`)
	hdr.Set("Content-Type", "image/jpeg")
	fw, err := mw.CreatePart(hdr)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write([]byte("\xff\xd8\xff\xd9")); err != nil { // minimal JPEG marker
		t.Fatalf("write image: %v", err)
	}
	mw.Close()

	req, err := http.NewRequest(method, srv.URL+path, &buf)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	return resp
}

// registerUser creates a user via the single-step register flow and returns the token.
func registerUser(t *testing.T, email, password string) string {
	t.Helper()
	resp := request(t, http.MethodPost, "/auth/register", map[string]string{
		"email": email, "password": password,
	}, "")
	assertStatus(t, resp, http.StatusCreated)
	var body struct {
		Token string `json:"token"`
	}
	decodeBody(t, resp, &body)
	if body.Token == "" {
		t.Fatal("registerUser: empty token")
	}
	return body.Token
}

// createCompletedSession creates a workout and logs a completed session on date,
// returning the session id.
func createCompletedSession(t *testing.T, token string, date time.Time) string {
	t.Helper()
	resp := request(t, http.MethodPost, "/workouts", map[string]any{
		"name": "Leg day", "days_of_week": []string{"monday"},
	}, token)
	assertStatus(t, resp, http.StatusCreated)
	var wk struct {
		ID string `json:"id"`
	}
	decodeBody(t, resp, &wk)

	resp = request(t, http.MethodPost, "/workout-sessions", map[string]any{
		"workout_id": wk.ID,
		"date":       date.Format(time.RFC3339),
		"status":     "complete",
	}, token)
	assertStatus(t, resp, http.StatusCreated)
	var sess struct {
		ID string `json:"id"`
	}
	decodeBody(t, resp, &sess)
	return sess.ID
}

// mondayOfThisWeek returns Monday and Tuesday (noon, SP tz) of the current week,
// both guaranteed inside the ranking window whatever day the test runs.
func weekDates(t *testing.T) (monday, tuesday time.Time) {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	daysFromMonday := (int(now.Weekday()) + 6) % 7
	monday = time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc).AddDate(0, 0, -daysFromMonday)
	return monday, monday.AddDate(0, 0, 1)
}

func TestGroupFlow(t *testing.T) {
	truncate(t)
	monday, tuesday := weekDates(t)

	ownerTok := registerUser(t, "owner@example.com", "password123")
	memberTok := registerUser(t, "member@example.com", "password123")

	// Owner creates a group.
	resp := request(t, http.MethodPost, "/groups", map[string]string{"name": "Hunters"}, ownerTok)
	assertStatus(t, resp, http.StatusCreated)
	var grp struct {
		ID         string `json:"id"`
		InviteCode string `json:"invite_code"`
		OwnerID    string `json:"owner_id"`
	}
	decodeBody(t, resp, &grp)
	if grp.InviteCode == "" {
		t.Fatal("expected an invite code")
	}

	// Member joins by code.
	resp = request(t, http.MethodPost, "/groups/join", map[string]string{"invite_code": grp.InviteCode}, memberTok)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Invalid code is rejected.
	resp = request(t, http.MethodPost, "/groups/join", map[string]string{"invite_code": "ZZZZZZ"}, memberTok)
	assertStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()

	// Both train Monday -> 1 point each.
	createCompletedSession(t, ownerTok, monday)
	createCompletedSession(t, memberTok, monday)
	// Owner trains again Monday (same day -> no extra point) and Tuesday (+1).
	createCompletedSession(t, ownerTok, monday)
	createCompletedSession(t, ownerTok, tuesday)

	// Ranking: owner 2, member 1, ordered desc.
	resp = request(t, http.MethodGet, "/groups/"+grp.ID+"/ranking", nil, ownerTok)
	assertStatus(t, resp, http.StatusOK)
	var ranking []struct {
		UserID string `json:"user_id"`
		Points int    `json:"points"`
	}
	decodeBody(t, resp, &ranking)
	if len(ranking) != 2 {
		t.Fatalf("ranking length = %d, want 2", len(ranking))
	}
	if ranking[0].Points != 2 {
		t.Errorf("top points = %d, want 2", ranking[0].Points)
	}
	if ranking[1].Points != 1 {
		t.Errorf("second points = %d, want 1", ranking[1].Points)
	}

	// Group detail header: top score and my score.
	resp = request(t, http.MethodGet, "/groups/"+grp.ID, nil, memberTok)
	assertStatus(t, resp, http.StatusOK)
	var detail struct {
		TopScore int `json:"top_score"`
		MyScore  int `json:"my_score"`
	}
	decodeBody(t, resp, &detail)
	if detail.TopScore != 2 {
		t.Errorf("top_score = %d, want 2", detail.TopScore)
	}
	if detail.MyScore != 1 {
		t.Errorf("my_score (member) = %d, want 1", detail.MyScore)
	}

	// Feed returns the members' completed sessions.
	resp = request(t, http.MethodGet, "/groups/"+grp.ID+"/feed", nil, memberTok)
	assertStatus(t, resp, http.StatusOK)
	var feed struct {
		Data []struct {
			WorkoutName string `json:"workout_name"`
			Name        string `json:"name"`
		} `json:"data"`
	}
	decodeBody(t, resp, &feed)
	if len(feed.Data) != 4 {
		t.Errorf("feed items = %d, want 4", len(feed.Data))
	}

	// Non-member cannot read the group.
	otherTok := registerUser(t, "outsider@example.com", "password123")
	resp = request(t, http.MethodGet, "/groups/"+grp.ID, nil, otherTok)
	assertStatus(t, resp, http.StatusForbidden)
	resp.Body.Close()

	// Member leaves.
	resp = request(t, http.MethodDelete, "/groups/"+grp.ID+"/leave", nil, memberTok)
	assertStatus(t, resp, http.StatusNoContent)
	resp.Body.Close()

	resp = request(t, http.MethodGet, "/groups/"+grp.ID, nil, memberTok)
	assertStatus(t, resp, http.StatusForbidden)
	resp.Body.Close()
}

func TestSessionPhotoAndCover(t *testing.T) {
	truncate(t)
	monday, _ := weekDates(t)

	ownerTok := registerUser(t, "photo-owner@example.com", "password123")
	memberTok := registerUser(t, "photo-member@example.com", "password123")

	sessionID := createCompletedSession(t, ownerTok, monday)

	// Attach a photo to the session -> photo_url is persisted.
	resp := uploadImage(t, http.MethodPost, "/workout-sessions/"+sessionID+"/photo", ownerTok)
	assertStatus(t, resp, http.StatusOK)
	var sess struct {
		PhotoURL *string `json:"photo_url"`
	}
	decodeBody(t, resp, &sess)
	if sess.PhotoURL == nil || *sess.PhotoURL == "" {
		t.Fatal("expected photo_url to be set after upload")
	}

	// A group so we can exercise the cover endpoint.
	resp = request(t, http.MethodPost, "/groups", map[string]string{"name": "Shadows"}, ownerTok)
	assertStatus(t, resp, http.StatusCreated)
	var grp struct {
		ID         string `json:"id"`
		InviteCode string `json:"invite_code"`
	}
	decodeBody(t, resp, &grp)

	resp = request(t, http.MethodPost, "/groups/join", map[string]string{"invite_code": grp.InviteCode}, memberTok)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	// Owner sets the cover -> cover_url persisted.
	resp = uploadImage(t, http.MethodPatch, "/groups/"+grp.ID+"/cover", ownerTok)
	assertStatus(t, resp, http.StatusOK)
	var withCover struct {
		CoverURL *string `json:"cover_url"`
	}
	decodeBody(t, resp, &withCover)
	if withCover.CoverURL == nil || *withCover.CoverURL == "" {
		t.Fatal("expected cover_url to be set after upload")
	}

	// A non-owner member cannot set the cover.
	resp = uploadImage(t, http.MethodPatch, "/groups/"+grp.ID+"/cover", memberTok)
	assertStatus(t, resp, http.StatusForbidden)
	resp.Body.Close()
}
