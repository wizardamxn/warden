package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"warden/internal/core"
	"warden/internal/jobs"
	"warden/internal/notes"
	"warden/migrations"
)

func video(i int) string { return fmt.Sprintf("video%06d", i) }

// sessionToken is the cookie a browser holds.
func (b *browser) sessionToken() string {
	b.t.Helper()
	address, err := url.Parse(b.server.URL)
	if err != nil {
		b.t.Fatal(err)
	}
	for _, cookie := range b.client.Jar.Cookies(address) {
		if cookie.Name == "warden_session" {
			return cookie.Value
		}
	}
	return ""
}

func TestSignupLoginLogout(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	server := newServer(t, q, core.Settings{})

	// Signup sets the cookie and answers with the user.
	b := newBrowser(t, server)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/auth/signup",
		strings.NewReader(`{"email":"  Student@Example.com ","password":"`+password+`","template":"general"}`))
	if err != nil {
		t.Fatal(err)
	}
	response, err := b.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	cookie := response.Header.Get("Set-Cookie")
	for _, want := range []string{"warden_session=", "Path=/", "Max-Age=2592000", "HttpOnly", "SameSite=Lax"} {
		if !strings.Contains(cookie, want) {
			t.Errorf("cookie %q is missing %s", cookie, want)
		}
	}
	if strings.Contains(cookie, "Secure") {
		t.Errorf("on 127.0.0.1 the cookie can't be Secure: %q", cookie)
	}
	user := b.me()
	if response.StatusCode != http.StatusCreated || user.Email != "student@example.com" || user.Template != "general" || user.NotesToday != 0 || user.DailyLimit != 3 || user.ID == 0 {
		t.Fatalf("signup: %d %+v", response.StatusCode, user)
	}

	// Only the token's hash is stored, and the password only as argon2id.
	token := b.sessionToken()
	sum := sha256.Sum256([]byte(token))
	var byToken, byHash int
	if err := q.DB.QueryRow(ctx, `SELECT count(*) FILTER (WHERE token_hash = $1), count(*) FILTER (WHERE token_hash = $2) FROM sessions`,
		token, hex.EncodeToString(sum[:])).Scan(&byToken, &byHash); err != nil {
		t.Fatal(err)
	}
	if token == "" || byToken != 0 || byHash != 1 {
		t.Errorf("token %q: stored as itself %d, as its hash %d", token, byToken, byHash)
	}
	var hash string
	if err := q.DB.QueryRow(ctx, `SELECT password_hash FROM users`).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") || strings.Contains(hash, password) {
		t.Errorf("password stored as %q", hash)
	}

	// The same email, in any case, can't sign up twice.
	other := newBrowser(t, server)
	code, data := other.send(http.MethodPost, "/v1/auth/signup", `{"email":"STUDENT@example.com","password":"`+password+`","template":"neet"}`)
	if code != http.StatusConflict || errorOf(t, data) == "" || other.sessionToken() != "" {
		t.Errorf("second signup: %d %s", code, data)
	}

	// Logout ends the session on the server, not only in the browser.
	if code, _ := b.send(http.MethodPost, "/v1/auth/logout", ""); code != http.StatusNoContent {
		t.Errorf("logout: %d", code)
	}
	if b.sessionToken() != "" {
		t.Error("logout must clear the cookie")
	}
	stolen := newBrowser(t, server)
	stolen.client.Jar.SetCookies(mustURL(t, server.URL), []*http.Cookie{{Name: "warden_session", Value: token}})
	if code, data := stolen.send(http.MethodGet, "/v1/me", ""); code != http.StatusUnauthorized || errorOf(t, data) != "Log in to continue." {
		t.Errorf("old token after logout: %d %s", code, data)
	}

	// Login, with the email in any case and spacing.
	code, data = b.send(http.MethodPost, "/v1/auth/login", `{"email":"STUDENT@example.com ","password":"`+password+`"}`)
	if code != http.StatusOK || readUser(t, data).Email != "student@example.com" || b.sessionToken() == "" || b.sessionToken() == token {
		t.Fatalf("login: %d %s", code, data)
	}
	if b.me().Template != "general" {
		t.Error("login lost the template")
	}

	// A session past its 30 days ends.
	if _, err := q.DB.Exec(ctx, `UPDATE sessions SET expires_at = now() - interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if code, _ := b.send(http.MethodGet, "/v1/me", ""); code != http.StatusUnauthorized {
		t.Errorf("expired session: %d", code)
	}

	// A made-up token is nobody's.
	forged := newBrowser(t, server)
	forged.client.Jar.SetCookies(mustURL(t, server.URL), []*http.Cookie{{Name: "warden_session", Value: strings.Repeat("A", 26)}})
	if code, _ := forged.send(http.MethodGet, "/v1/me", ""); code != http.StatusUnauthorized {
		t.Errorf("forged token: %d", code)
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestWrongPasswordAndThrottling(t *testing.T) {
	q := testQueue(t)
	server := newServer(t, q, core.Settings{})
	signUp(t, server, "student@example.com", "neet")
	login := func(b *browser, email, pass string) (int, string) {
		code, data := b.send(http.MethodPost, "/v1/auth/login", `{"email":"`+email+`","password":"`+pass+`"}`)
		return code, errorOf(t, data)
	}

	// A wrong password and an unknown email look the same.
	b := newBrowser(t, server)
	b.ip = "198.51.100.1"
	_, wrongPassword := login(b, "student@example.com", "wrong password")
	code, unknownEmail := login(b, "nobody@example.com", password)
	if code != http.StatusUnauthorized || wrongPassword != unknownEmail || wrongPassword != "Wrong email or password." {
		t.Errorf("%d %q %q", code, wrongPassword, unknownEmail)
	}

	// 5 wrong passwords for one email, from anywhere, block that email, even with
	// the right password. One was above.
	for i := range 4 {
		b.ip = fmt.Sprintf("198.51.100.%d", 10+i)
		if code, _ := login(b, "student@example.com", "wrong password"); code != http.StatusUnauthorized {
			t.Fatalf("try %d: %d", i, code)
		}
	}
	if code, message := login(b, "student@example.com", password); code != http.StatusTooManyRequests || !strings.Contains(message, "15 minutes") {
		t.Errorf("after 5 wrong passwords: %d %q", code, message)
	}
	if b.sessionToken() != "" {
		t.Error("a blocked login must not set a session")
	}

	// 20 wrong passwords from one address block it, for any email.
	attacker := newBrowser(t, server)
	attacker.ip = "203.0.113.9"
	for i := range 20 {
		if code, _ := login(attacker, fmt.Sprintf("guess%d@example.com", i), "wrong password"); code != http.StatusUnauthorized {
			t.Fatalf("guess %d: %d", i, code)
		}
	}
	signUp(t, server, "other@example.com", "neet")
	if code, _ := login(attacker, "other@example.com", password); code != http.StatusTooManyRequests {
		t.Errorf("blocked address: %d", code)
	}
	// Someone else is not blocked by it.
	friend := newBrowser(t, server)
	friend.ip = "203.0.113.10"
	if code, _ := login(friend, "other@example.com", password); code != http.StatusOK {
		t.Errorf("another address: %d", code)
	}
}

func TestTemplateChangeAffectsOnlyNewNotes(t *testing.T) {
	q := testQueue(t)
	server := newServer(t, q, core.Settings{})
	student := signUp(t, server, "student@example.com", "neet")
	student.add(video(1))

	code, data := student.send(http.MethodPatch, "/v1/me", `{"template":"jee"}`)
	if code != http.StatusBadRequest || errorOf(t, data) == "" {
		t.Errorf("unknown template: %d %s", code, data)
	}
	code, data = student.send(http.MethodPatch, "/v1/me", `{"template":"general"}`)
	if code != http.StatusOK || readUser(t, data).Template != "general" || readUser(t, data).NotesToday != 1 {
		t.Fatalf("patch: %d %s", code, data)
	}

	// The lecture added before stays NEET.
	if _, view, _ := student.note(http.MethodGet, "/v1/notes/"+video(1), ""); view.VideoID != video(1) || templateOf(t, q, video(1)) != "neet" {
		t.Errorf("old lecture = %+v", view)
	}
	student.add(video(2))
	if got := templateOf(t, q, video(2)); got != "general" {
		t.Errorf("new lecture template = %q", got)
	}

	// Added again under General, the lecture opens as the newer one.
	student.add(video(1))
	var count int
	if err := q.DB.QueryRow(context.Background(), `SELECT count(*) FROM library WHERE video_id = $1`, video(1)).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("library rows = %d", count)
	}
	if _, err := q.DB.Exec(context.Background(), `UPDATE library SET added_at = added_at - interval '1 hour' WHERE template = 'neet'`); err != nil {
		t.Fatal(err)
	}
	template, err := q.LibraryTemplate(context.Background(), student.me().ID, video(1))
	if err != nil || template != "general" {
		t.Errorf("template = %q, %v", template, err)
	}
}

// templateOf reads the only job a video has.
func templateOf(t *testing.T, q *jobs.Queue, videoID string) string {
	t.Helper()
	var template string
	if err := q.DB.QueryRow(context.Background(), `SELECT template FROM jobs WHERE video_id = $1`, videoID).Scan(&template); err != nil {
		t.Fatal(err)
	}
	return template
}

func TestDailyLimit(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	client, _ := fakeGemini(t)
	server := newServer(t, q, core.Settings{})
	student := signUp(t, server, "student@example.com", "neet")

	// Another student's finished lecture still counts: every new lecture does.
	seedVideo(t, q, video(1))
	runNext(t, q, client)
	if code, view, _ := student.add(video(1)); code != http.StatusOK || view.Status != "done" {
		t.Fatalf("reused lecture: %d %+v", code, view)
	}
	for i := 2; i <= 3; i++ {
		if code, _, _ := student.add(video(i)); code != http.StatusAccepted {
			t.Fatalf("lecture %d: %d", i, code)
		}
	}
	if got := student.me().NotesToday; got != 3 {
		t.Errorf("notes_today = %d", got)
	}

	code, data := student.send(http.MethodPost, "/v1/notes", `{"url":"https://youtu.be/`+video(4)+`"}`)
	if code != http.StatusTooManyRequests || errorOf(t, data) != "You've made 3 notes today. More tomorrow." {
		t.Errorf("fourth lecture: %d %s", code, data)
	}
	var jobCount int
	if err := q.DB.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE video_id = $1`, video(4)).Scan(&jobCount); err != nil {
		t.Fatal(err)
	}
	if jobCount != 0 {
		t.Error("a refused lecture must not make a job")
	}

	// A lecture already in the library opens again, and doesn't count.
	if code, view, _ := student.add(video(2)); code != http.StatusAccepted || view.VideoID != video(2) {
		t.Errorf("re-adding: %d %+v", code, view)
	}

	// Try again doesn't count either.
	seedCaptions(t, q, video(2))
	if _, err := q.DB.Exec(ctx, `UPDATE job_steps s SET status = 'failed', error = 'Plain words.' FROM jobs j
		WHERE s.job_id = j.id AND j.video_id = $1 AND s.name = 'notes'`, video(2)); err != nil {
		t.Fatal(err)
	}
	if code, view, _ := student.note(http.MethodPost, "/v1/notes/"+video(2)+"/retry", ""); code != http.StatusAccepted || view.Steps[1].Status != "pending" {
		t.Errorf("retry at the limit: %d %+v", code, view)
	}
	if got := student.me().NotesToday; got != 3 {
		t.Errorf("notes_today after re-adding and retrying = %d", got)
	}

	// Yesterday's lectures don't count today, India time.
	if _, err := q.DB.Exec(ctx, `UPDATE library SET added_at = (date_trunc('day', now() AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata') - interval '1 second'
		WHERE video_id = $1`, video(1)); err != nil {
		t.Fatal(err)
	}
	if got := student.me().NotesToday; got != 2 {
		t.Errorf("notes_today after a lecture moved to yesterday = %d", got)
	}
	if code, _, _ := student.add(video(4)); code != http.StatusAccepted {
		t.Errorf("a new day's lecture: %d", code)
	}
}

func TestTwoQuickRequestsAtTwoOfThree(t *testing.T) {
	q := testQueue(t)
	server := newServer(t, q, core.Settings{})
	for round := range 5 {
		student := signUp(t, server, fmt.Sprintf("student%d@example.com", round), "neet")
		student.add(video(round*10 + 1))
		student.add(video(round*10 + 2))

		codes := make([]int, 2)
		var requests sync.WaitGroup
		for i := range codes {
			requests.Go(func() {
				codes[i], _, _ = student.add(video(round*10 + 3 + i))
			})
		}
		requests.Wait()
		slices.Sort(codes)
		if codes[0] != http.StatusAccepted || codes[1] != http.StatusTooManyRequests {
			t.Errorf("round %d: codes = %v, want one 202 and one 429", round, codes)
		}
		if got := student.me().NotesToday; got != 3 {
			t.Errorf("round %d: notes_today = %d", round, got)
		}
	}
}

func TestManyQuickRequestsAddOnlyThree(t *testing.T) {
	q := testQueue(t)
	student := signUp(t, newServer(t, q, core.Settings{}), "student@example.com", "neet")
	var mu sync.Mutex
	passed := 0
	var requests sync.WaitGroup
	for i := range 10 {
		requests.Go(func() {
			if code, _, _ := student.add(video(i)); code == http.StatusAccepted {
				mu.Lock()
				passed++
				mu.Unlock()
			}
		})
	}
	requests.Wait()
	if passed != 3 || student.me().NotesToday != 3 {
		t.Errorf("passed = %d", passed)
	}
}

func TestTotalCap(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	server := newServer(t, q, core.Settings{TotalDailyCap: 2})
	first := signUp(t, server, "first@example.com", "neet")
	second := signUp(t, server, "second@example.com", "neet")

	first.add(video(1))
	first.add(video(2))
	code, data := second.send(http.MethodPost, "/v1/notes", `{"url":"https://youtu.be/`+video(3)+`"}`)
	if code != http.StatusServiceUnavailable || errorOf(t, data) != "Warden is at its limit for today. Try again tomorrow." {
		t.Errorf("past the cap: %d %s", code, data)
	}
	if got := second.me().NotesToday; got != 0 {
		t.Errorf("a refused lecture counted: notes_today = %d", got)
	}
	// A lecture that already has a job costs nothing more, so the cap lets it through.
	if code, _, _ := second.add(video(1)); code != http.StatusAccepted {
		t.Errorf("existing job: %d", code)
	}
	// The same lecture under another template is a new job, so the cap refuses it.
	if code, _ := second.send(http.MethodPatch, "/v1/me", `{"template":"general"}`); code != http.StatusOK {
		t.Fatal(code)
	}
	if code, _ := second.send(http.MethodPost, "/v1/notes", `{"url":"https://youtu.be/`+video(1)+`"}`); code != http.StatusServiceUnavailable {
		t.Errorf("new template, new job, past the cap: %d", code)
	}

	// Jobs from before today don't count.
	if _, err := q.DB.Exec(ctx, `UPDATE jobs SET created_at = now() - interval '1 day'`); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := second.add(video(3)); code != http.StatusAccepted {
		t.Errorf("a new day: %d", code)
	}

	// Many new students racing for the cap's last two places: only two get them.
	if _, err := q.DB.Exec(ctx, `UPDATE jobs SET created_at = now() - interval '1 day'`); err != nil {
		t.Fatal(err)
	}
	var racers []*browser
	for i := range 8 {
		racers = append(racers, signUp(t, server, fmt.Sprintf("racer%d@example.com", i), "neet"))
	}
	codes := make([]int, len(racers))
	var requests sync.WaitGroup
	for i, b := range racers {
		requests.Go(func() {
			codes[i], _ = b.send(http.MethodPost, "/v1/notes", `{"url":"https://youtu.be/`+video(10+i)+`"}`)
		})
	}
	requests.Wait()
	slices.Sort(codes)
	if want := []int{202, 202, 503, 503, 503, 503, 503, 503}; !slices.Equal(codes, want) {
		t.Errorf("race for the last places: %v", codes)
	}
}

func TestOneNotePerVideoAndTemplate(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	client, fake := fakeGemini(t)
	server := newServer(t, q, core.Settings{})
	neet := signUp(t, server, "neet@example.com", "neet")
	general := signUp(t, server, "general@example.com", "general")
	alsoNEET := signUp(t, server, "also@example.com", "neet")

	neet.add(testID)
	general.add(testID)
	alsoNEET.add(testID)
	var count int
	if err := q.DB.QueryRow(ctx, `SELECT count(*) FROM jobs WHERE video_id = $1`, testID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("jobs = %d, want one per template", count)
	}

	seedCaptions(t, q, testID)
	runNext(t, q, client)
	runNext(t, q, client)
	wantNothingToClaim(t, q)
	fake.wantCalls(t, map[string]int{"notes": 2, "flashcards": 2, "quiz": 2})

	for _, tt := range []struct {
		b         *browser
		template  string
		quizTypes []string
	}{
		{neet, "neet", []string{"assertion-reason", "direct", "match", "statement"}},
		{general, "general", []string{"direct", "statement"}},
		{alsoNEET, "neet", []string{"assertion-reason", "direct", "match", "statement"}},
	} {
		code, view, _ := tt.b.note(http.MethodGet, "/v1/notes/"+testID, "")
		if code != http.StatusOK || view.Status != "done" || view.Note == nil || view.Note.Template != tt.template {
			t.Fatalf("%s: %d %+v", tt.template, code, view)
		}
		var types []string
		for _, question := range view.Note.Quiz {
			if !slices.Contains(types, question.Type) {
				types = append(types, question.Type)
			}
		}
		slices.Sort(types)
		if !slices.Equal(types, tt.quizTypes) {
			t.Errorf("%s quiz types = %v", tt.template, types)
		}
	}

	// A fourth student reuses the General note with no AI calls.
	if code, view, _ := signUp(t, server, "fourth@example.com", "general").add(testID); code != http.StatusOK || view.Note.Template != "general" {
		t.Errorf("reuse: %d %+v", code, view)
	}
	fake.wantCalls(t, map[string]int{"notes": 2, "flashcards": 2, "quiz": 2})
}

func TestGeneralQuizRefusesNEETTypes(t *testing.T) {
	q := testQueue(t)
	client, fake := fakeGemini(t)
	student := signUp(t, newServer(t, q, core.Settings{}), "general@example.com", "general")
	student.add(testID)
	seedCaptions(t, q, testID)
	fake.neetQuizOnly = true

	runNext(t, q, client)
	_, view, _ := student.note(http.MethodGet, "/v1/notes/"+testID, "")
	if view.Status != "partial" || view.Steps[3].Status != "failed" || len(view.Note.Quiz) != 0 {
		t.Errorf("view = %+v", view)
	}
	// Asked once, then once more with the problems.
	fake.wantCalls(t, map[string]int{"notes": 1, "flashcards": 1, "quiz": 2})
}

func TestRevise(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	client, fake := fakeGemini(t)
	student := signUp(t, newServer(t, q, core.Settings{}), "student@example.com", "neet")

	for i := 1; i <= 3; i++ {
		student.add(video(i))
		seedCaptions(t, q, video(i))
		runNext(t, q, client)
	}
	// video 3's flashcards failed, so it has none to show.
	fake.fail(500, "failed", "flashcards")
	student.send(http.MethodPost, "/v1/notes/"+video(3)+"/retry", "")
	if _, err := q.DB.Exec(ctx, `UPDATE job_steps s SET status = 'pending' FROM jobs j WHERE s.job_id = j.id AND j.video_id = $1 AND s.name = 'flashcards'`, video(3)); err != nil {
		t.Fatal(err)
	}
	runNext(t, q, client)

	for _, change := range []struct {
		videoID string
		age     string
	}{
		{video(1), "2 days 1 hour"},
		{video(2), "1 day 23 hours"}, // not yet
		{video(3), "5 days"},
	} {
		if _, err := q.DB.Exec(ctx, `UPDATE library SET added_at = now() - $2::interval WHERE video_id = $1`, change.videoID, change.age); err != nil {
			t.Fatal(err)
		}
	}
	student.add(video(4)) // today's, and not made yet

	code, data := student.send(http.MethodGet, "/v1/revise", "")
	var body struct {
		Cards []jobs.Card `json:"cards"`
	}
	if err := json.Unmarshal(data, &body); err != nil || code != http.StatusOK {
		t.Fatalf("%d %s", code, data)
	}
	if len(body.Cards) != 10 {
		t.Fatalf("cards = %d, want video 1's 10", len(body.Cards))
	}
	first := body.Cards[0]
	if first.VideoID != video(1) || first.Title != "Immunity" || first.Template != "neet" || first.Question != "What is immunity, part 1?" || first.Answer == "" || first.AddedAt.IsZero() {
		t.Errorf("card = %+v", first)
	}
	if !strings.Contains(string(data), `"start_seconds":0`) || !strings.Contains(string(data), `"added_at":"`) {
		t.Errorf("body = %s", data)
	}

	// Oldest lecture first.
	if _, err := q.DB.Exec(ctx, `UPDATE library SET added_at = now() - interval '3 days' WHERE video_id = $1`, video(2)); err != nil {
		t.Fatal(err)
	}
	_, data = student.send(http.MethodGet, "/v1/revise", "")
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Cards) != 20 || body.Cards[0].VideoID != video(2) || body.Cards[10].VideoID != video(1) {
		t.Errorf("order: %d cards, first %s, eleventh %s", len(body.Cards), body.Cards[0].VideoID, body.Cards[10].VideoID)
	}
}

func TestLibrary(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	client, _ := fakeGemini(t)
	student := signUp(t, newServer(t, q, core.Settings{}), "student@example.com", "neet")

	code, data := student.send(http.MethodGet, "/v1/library", "")
	if code != http.StatusOK || strings.TrimSpace(string(data)) != `{"notes":[]}` {
		t.Errorf("empty library: %d %s", code, data)
	}

	student.add(video(1))
	seedCaptions(t, q, video(1))
	runNext(t, q, client)
	if _, err := q.DB.Exec(ctx, `UPDATE library SET added_at = now() - interval '1 hour'`); err != nil {
		t.Fatal(err)
	}
	student.add(video(2))

	_, data = student.send(http.MethodGet, "/v1/library", "")
	var body struct {
		Notes []jobs.Lecture `json:"notes"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Notes) != 2 {
		t.Fatalf("library = %s", data)
	}
	newest, done := body.Notes[0], body.Notes[1]
	if newest.VideoID != video(2) || newest.Title != "" || newest.Status != "queued" || newest.Template != "neet" {
		t.Errorf("newest = %+v", newest)
	}
	if done.VideoID != video(1) || done.Title != "Immunity" || done.Channel != "Test" || done.DurationSeconds != 100 || done.Status != "done" || done.AddedAt.IsZero() {
		t.Errorf("finished = %+v", done)
	}
}

// One student never sees another's library, notes or cards, even knowing the video ID.
func TestStudentsCannotSeeEachOther(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	client, _ := fakeGemini(t)
	server := newServer(t, q, core.Settings{})
	alice := signUp(t, server, "alice@example.com", "neet")
	bob := signUp(t, server, "bob@example.com", "neet")

	alice.add(testID)
	seedCaptions(t, q, testID)
	runNext(t, q, client)
	if _, err := q.DB.Exec(ctx, `UPDATE library SET added_at = now() - interval '3 days'`); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/v1/notes/" + testID, "/v1/notes/" + testID + "/retry"} {
		method := http.MethodGet
		if strings.HasSuffix(path, "retry") {
			method = http.MethodPost
		}
		if code, data := bob.send(method, path, ""); code != http.StatusNotFound || strings.Contains(string(data), "Immunity") {
			t.Errorf("bob %s %s: %d %s", method, path, code, data)
		}
	}
	if _, data := bob.send(http.MethodGet, "/v1/library", ""); strings.TrimSpace(string(data)) != `{"notes":[]}` {
		t.Errorf("bob's library = %s", data)
	}
	if _, data := bob.send(http.MethodGet, "/v1/revise", ""); strings.TrimSpace(string(data)) != `{"cards":[]}` {
		t.Errorf("bob's revise = %s", data)
	}

	// Bob's own lecture doesn't show up for Alice either.
	bob.add(video(9))
	_, data := alice.send(http.MethodGet, "/v1/library", "")
	if strings.Contains(string(data), video(9)) || !strings.Contains(string(data), testID) {
		t.Errorf("alice's library = %s", data)
	}
	if code, _ := alice.send(http.MethodGet, "/v1/notes/"+video(9), ""); code != http.StatusNotFound {
		t.Errorf("alice reading bob's lecture: %d", code)
	}
	if code, _ := alice.send(http.MethodGet, "/v1/revise", ""); code != http.StatusOK {
		t.Error(code)
	}
}

func TestSampleLecture(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	_, fake := fakeGemini(t)

	data, err := os.ReadFile("../../../apps/web/fixtures/k7Zb1iOV-ys.json")
	if err != nil {
		t.Fatal(err)
	}
	var note notes.Note
	if err := json.Unmarshal(data, &note); err != nil {
		t.Fatal(err)
	}
	if err := q.SaveFinished(ctx, note.VideoID, "neet", note); err != nil {
		t.Fatal(err)
	}
	// Loading it twice just replaces it.
	if err := q.SaveFinished(ctx, note.VideoID, "neet", note); err != nil {
		t.Fatal(err)
	}

	server := newServer(t, q, core.Settings{SampleVideoID: note.VideoID})
	guest := newBrowser(t, server)
	code, view, fields := guest.note(http.MethodGet, "/v1/sample", "")
	if code != http.StatusOK || view.Status != "done" || view.Note == nil || view.Note.Title != note.Title ||
		len(view.Note.Flashcards) != len(note.Flashcards) || len(view.Note.Quiz) != len(note.Quiz) || view.Note.Template != "neet" {
		t.Fatalf("sample: %d %+v", code, view)
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "null") {
		t.Errorf("sample has a null: %s", encoded)
	}

	// A student pasting the sample's link gets it at once, with no work queued.
	student := signUp(t, server, "student@example.com", "neet")
	if code, view, _ := student.add(note.VideoID); code != http.StatusOK || view.Status != "done" {
		t.Errorf("pasting the sample: %d %+v", code, view)
	}
	wantNothingToClaim(t, q)
	fake.wantCalls(t, map[string]int{})

	// A sample ID with no job is a plain 404.
	other := newServer(t, q, core.Settings{SampleVideoID: video(1)})
	if code, data := newBrowser(t, other).send(http.MethodGet, "/v1/sample", ""); code != http.StatusNotFound || errorOf(t, data) == "" {
		t.Errorf("missing sample: %d %s", code, data)
	}
}

func TestMigrationOnADatabaseWithJobsFrom001(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// The database as it was before accounts: 001 applied, one finished job.
	first, err := os.ReadFile("../../migrations/001_jobs.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, string(first)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `
		CREATE TABLE schema_migrations (name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
		INSERT INTO schema_migrations (name) VALUES ('001_jobs.sql');
		INSERT INTO jobs (video_id) VALUES ('`+testID+`');
		INSERT INTO job_steps (job_id, name, prompt_version, status, result)
		SELECT id, s.name, s.version, 'done', s.result::jsonb FROM jobs, (VALUES
			('captions', 'v1', '{"Video":{"ID":"`+testID+`","Title":"Immunity"},"Lines":[]}'),
			('notes', 'v4', '{"video_id":"`+testID+`","title":"Immunity","tldr":"Old note."}'),
			('flashcards', 'v2', '[]'),
			('quiz', 'v3', '[]')) AS s (name, version, result);`); err != nil {
		t.Fatal(err)
	}

	applied, err := migrations.Apply(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(applied, []string{"002_accounts.sql"}) {
		t.Errorf("applied = %v", applied)
	}

	// The old job is now the NEET job, and nothing needs remaking.
	q := jobs.New(db)
	view, err := q.Get(ctx, testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "done" || view.Note == nil || view.Note.TLDR != "Old note." || view.Note.Template != "neet" {
		t.Errorf("old job = %+v", view)
	}
	if view, err := q.Enqueue(ctx, testID, "neet"); err != nil || view.Status != "done" {
		t.Errorf("re-enqueued: %+v %v", view, err)
	}
	wantNothingToClaim(t, q)

	// The same video can now have a General job beside it, but not a second NEET one.
	if view, err := q.Enqueue(ctx, testID, "general"); err != nil || view.Status != "queued" {
		t.Errorf("general job: %+v %v", view, err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO jobs (video_id, template) VALUES ($1, 'neet')`, testID); err == nil {
		t.Error("a second NEET job for one video must be refused")
	}
	if _, err := db.Exec(ctx, `INSERT INTO jobs (video_id) VALUES ($1)`, "video000001"); err == nil {
		t.Error("a job must name its template")
	}
	if _, err := db.Exec(ctx, `INSERT INTO jobs (video_id, template) VALUES ($1, 'jee')`, "video000001"); err == nil {
		t.Error("an unknown template must be refused")
	}
	var secured int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM pg_class WHERE relname IN ('users', 'sessions', 'library') AND relrowsecurity
		AND relnamespace = current_schema()::regnamespace`).Scan(&secured); err != nil {
		t.Fatal(err)
	}
	if secured != 3 {
		t.Errorf("row level security is on for %d of the 3 new tables", secured)
	}
}
