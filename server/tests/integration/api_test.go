package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"warden/internal/api"
	"warden/internal/auth"
	"warden/internal/core"
	"warden/internal/jobs"
)

const password = "long enough password"

// newServer runs the API on a test's own schema.
func newServer(t *testing.T, q *jobs.Queue, settings core.Settings) *httptest.Server {
	t.Helper()
	settings.WebOrigin = core.LocalWebOrigin
	server := httptest.NewServer(api.New(q, auth.New(q.DB), settings))
	t.Cleanup(server.Close)
	return server
}

// browser is one student's browser: it keeps their session cookie.
type browser struct {
	t      *testing.T
	server *httptest.Server
	client *http.Client
	ip     string // sent as CF-Connecting-IP, as the tunnel would
}

func newBrowser(t *testing.T, server *httptest.Server) *browser {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Its own client: server.Client() is shared, and so would be the cookies.
	client := &http.Client{Transport: server.Client().Transport, Jar: jar}
	return &browser{t: t, server: server, client: client}
}

// signUp makes a new student and logs them in.
func signUp(t *testing.T, server *httptest.Server, email, template string) *browser {
	t.Helper()
	b := newBrowser(t, server)
	if code, body := b.send(http.MethodPost, "/v1/auth/signup", `{"email":"`+email+`","password":"`+password+`","template":"`+template+`"}`); code != http.StatusCreated {
		t.Fatalf("signup: %d %s", code, body)
	}
	return b
}

func (b *browser) send(method, path, body string) (int, []byte) {
	b.t.Helper()
	request, err := http.NewRequest(method, b.server.URL+path, strings.NewReader(body))
	if err != nil {
		b.t.Fatal(err)
	}
	if b.ip != "" {
		request.Header.Set("CF-Connecting-IP", b.ip)
	}
	response, err := b.client.Do(request)
	if err != nil {
		b.t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		b.t.Fatal(err)
	}
	return response.StatusCode, data
}

// note sends a request that answers with a job.
func (b *browser) note(method, path, body string) (int, jobs.View, map[string]any) {
	b.t.Helper()
	code, data := b.send(method, path, body)
	var view jobs.View
	var fields map[string]any
	if err := json.Unmarshal(data, &view); err != nil {
		b.t.Fatalf("%s %s: %v %s", method, path, err, data)
	}
	if err := json.Unmarshal(data, &fields); err != nil {
		b.t.Fatal(err)
	}
	return code, view, fields
}

// add pastes a lecture's link.
func (b *browser) add(videoID string) (int, jobs.View, map[string]any) {
	b.t.Helper()
	return b.note(http.MethodPost, "/v1/notes", `{"url":"https://www.youtube.com/watch?v=`+videoID+`"}`)
}

type userView struct {
	ID         int64  `json:"id"`
	Email      string `json:"email"`
	Template   string `json:"template"`
	NotesToday int    `json:"notes_today"`
	DailyLimit int    `json:"daily_limit"`
}

func (b *browser) me() userView {
	b.t.Helper()
	code, data := b.send(http.MethodGet, "/v1/me", "")
	if code != http.StatusOK {
		b.t.Fatalf("me: %d %s", code, data)
	}
	return readUser(b.t, data)
}

func readUser(t *testing.T, data []byte) userView {
	t.Helper()
	var body struct {
		User *userView `json:"user"`
	}
	if err := json.Unmarshal(data, &body); err != nil || body.User == nil {
		t.Fatalf("user: %v %s", err, data)
	}
	return *body.User
}

func errorOf(t *testing.T, data []byte) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("%v %s", err, data)
	}
	return body.Error
}

func TestAPIContract(t *testing.T) {
	q := testQueue(t)
	client, fake := fakeGemini(t)
	server := newServer(t, q, core.Settings{})
	student := signUp(t, server, "student@example.com", "neet")

	if code, _, fields := student.note(http.MethodGet, "/v1/notes/"+testID, ""); code != 404 || fields["error"] == "" {
		t.Errorf("unknown video: %d %v", code, fields)
	}
	if code, _, fields := student.note(http.MethodPost, "/v1/notes/"+testID+"/retry", ""); code != 404 || fields["error"] == "" {
		t.Errorf("unknown retry: %d %v", code, fields)
	}

	// First request: queued, nothing ready.
	code, view, fields := student.add(testID)
	if code != 202 || view.VideoID != testID || view.Status != "queued" || view.Note != nil {
		t.Fatalf("first post: %d %+v", code, view)
	}
	if note, present := fields["note"]; !present || note != nil {
		t.Errorf("note must be there and null: %v", fields)
	}
	wantSteps := []string{"captions", "notes", "flashcards", "quiz"}
	if len(view.Steps) != len(wantSteps) {
		t.Fatalf("steps = %+v", view.Steps)
	}
	for i, name := range wantSteps {
		if view.Steps[i].Name != name || view.Steps[i].Status != "pending" {
			t.Errorf("step %d = %+v", i, view.Steps[i])
		}
	}

	// The worker's part, with a failure.
	seedCaptions(t, q, testID)
	fake.fail(500, `{"error":{"message":"raw gemini text"}}`, "quiz")
	runNext(t, q, client)

	code, view, _ = student.note(http.MethodGet, "/v1/notes/"+testID, "")
	if code != 200 || view.Status != "partial" || view.Note == nil || view.Note.FailedSteps[0] != "quiz" || view.Note.Template != "neet" {
		t.Fatalf("get: %d %+v", code, view)
	}
	if err := view.Steps[3].Error; err == "" || strings.Contains(err, "raw gemini") {
		t.Errorf("quiz error = %q", err)
	}
	if view.Error != "" {
		t.Errorf("a partial note has no job error, got %q", view.Error)
	}

	// Retry queues only the failed step.
	fake.fail(0, "")
	code, view, _ = student.note(http.MethodPost, "/v1/notes/"+testID+"/retry", "")
	if code != 202 || view.Status != "queued" || view.Steps[3].Status != "pending" || view.Steps[2].Status != "done" {
		t.Fatalf("retry: %d %+v", code, view)
	}
	runNext(t, q, client)

	// A finished video answers 200 straight away, with no AI calls.
	code, view, fields = student.add(testID)
	if code != 200 || view.Status != "done" || view.Note == nil || len(view.Note.Quiz) != 5 {
		t.Fatalf("second post: %d %+v", code, view)
	}
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "null") || !strings.Contains(string(data), `"template":"neet"`) {
		t.Errorf("no field may be null once a note exists, and the note names its template: %s", data)
	}
	wantNothingToClaim(t, q)
	fake.wantCalls(t, map[string]int{"notes": 1, "flashcards": 1, "quiz": 2})
}

func TestCreateNoteRejectsBadInput(t *testing.T) {
	q := testQueue(t)
	student := signUp(t, newServer(t, q, core.Settings{}), "student@example.com", "neet")
	tests := []struct {
		name string
		body string
	}{
		{name: "not a youtube link", body: `{"url":"https://example.com/video"}`},
		{name: "no link", body: `{}`},
		{name: "not json", body: `hello`},
		{name: "empty", body: ``},
		{name: "too big", body: `{"url":"` + strings.Repeat("a", 4<<10) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, data := student.send(http.MethodPost, "/v1/notes", tt.body)
			if code != http.StatusBadRequest || errorOf(t, data) == "" {
				t.Errorf("%d %s", code, data)
			}
		})
	}
	if got := student.me().NotesToday; got != 0 {
		t.Errorf("a bad link counted: notes_today = %d", got)
	}
}
