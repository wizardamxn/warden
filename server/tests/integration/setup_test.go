package integration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"warden/internal/jobs"
	"warden/internal/notes"
	"warden/internal/pipeline/llm"
	"warden/migrations"
)

const (
	// The Postgres from deploy/compose.yaml. TEST_DATABASE_URL points somewhere
	// else. DATABASE_URL is never used, so tests can't reach a real database.
	localURL = "postgres://warden:warden@127.0.0.1:55432/warden"

	testID = "VjnHhmST8GY"
)

// testQueue gives a test its own new schema with the migrations applied, on a
// real Postgres, and drops it afterwards.
func testQueue(t *testing.T) *jobs.Queue {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = localURL
	}
	ctx := context.Background()

	dial, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	admin, err := pgx.Connect(dial, url)
	if err != nil {
		t.Skipf("SKIPPED, needs a real Postgres. Start the local one with: docker compose -f deploy/compose.yaml up -d --wait. Connecting said: %v", err)
	}
	t.Cleanup(func() { admin.Close(ctx) })

	schema := "test_" + strings.ToLower(rand.Text())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	})

	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)

	if _, err := migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	return jobs.New(db)
}

// seedVideo adds a job with its captions already saved, so tests never run yt-dlp.
func seedVideo(t *testing.T, q *jobs.Queue, videoID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := q.Enqueue(ctx, videoID); err != nil {
		t.Fatal(err)
	}
	_, err := q.DB.Exec(ctx, `
		UPDATE job_steps SET status = 'done', result = $2
		FROM jobs j WHERE job_steps.job_id = j.id AND j.video_id = $1 AND name = 'captions'`, videoID,
		`{"Video":{"ID":"`+videoID+`","Title":"Immunity","DurationSeconds":100,"Source":"auto-captions"},"Lines":[{"StartSeconds":0,"Text":"immunity"}]}`)
	if err != nil {
		t.Fatal(err)
	}
}

// stepOf reads one step's row, as the assertions need it.
type storedStep struct {
	Status      string
	Attempts    int
	HasResult   bool
	Error       string
	InputTokens int
}

func stepOf(t *testing.T, q *jobs.Queue, videoID, name string) storedStep {
	t.Helper()
	var s storedStep
	err := q.DB.QueryRow(context.Background(), `
		SELECT s.status, s.attempts, s.result IS NOT NULL, s.error, s.input_tokens
		FROM job_steps s JOIN jobs j ON j.id = s.job_id
		WHERE j.video_id = $1 AND s.name = $2`, videoID, name).
		Scan(&s.Status, &s.Attempts, &s.HasResult, &s.Error, &s.InputTokens)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// fakeAPI stands in for Gemini. It counts calls by step. Steps in failing get
// an error, and hold, if set, keeps every call waiting until it's closed.
type fakeAPI struct {
	mu      sync.Mutex
	failing map[string]int // step to the HTTP status it gets
	body    string
	calls   map[string]int
	hold    chan struct{}
}

func (f *fakeAPI) fail(status int, body string, steps ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing = map[string]int{}
	f.body = body
	for _, step := range steps {
		f.failing[step] = status
	}
}

func (f *fakeAPI) counts() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	counts := map[string]int{}
	for step, calls := range f.calls {
		counts[step] = calls
	}
	return counts
}

func (f *fakeAPI) wantCalls(t *testing.T, want map[string]int) {
	t.Helper()
	got := f.counts()
	for _, step := range []string{"notes", "flashcards", "quiz"} {
		if got[step] != want[step] {
			t.Errorf("%s calls = %d, want %d", step, got[step], want[step])
		}
	}
}

func fakeAnswers(t *testing.T) map[string]string {
	t.Helper()
	cards := make([]notes.Flashcard, 10)
	for i := range cards {
		cards[i] = notes.Flashcard{Question: "What is immunity?", Answer: "Resistance to disease."}
	}
	quiz := make([]notes.Question, 5)
	for i, kind := range []string{"direct", "statement", "assertion-reason", "match", "direct"} {
		quiz[i] = notes.Question{
			Type:        kind,
			Question:    "Statement I: Immunity is resistance.\nStatement II: It is sleep.",
			Options:     []string{"Resistance", "Sleep", "Growth", "Digestion"},
			Explanation: "It is resistance to disease, not sleep.",
		}
	}
	cardsJSON, err := json.Marshal(cards)
	if err != nil {
		t.Fatal(err)
	}
	quizJSON, err := json.Marshal(quiz)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"notes":      `{"tldr":"Immunity.","key_points":["Two types."],"sections":[{"start_seconds":0,"title":"Immunity","bullets":["Two types."],"exam_mark":"","exam_quote":""}]}`,
		"flashcards": string(cardsJSON),
		"quiz":       string(quizJSON),
	}
}

func fakeGemini(t *testing.T) (*llm.Client, *fakeAPI) {
	t.Helper()
	answers := fakeAnswers(t)
	fake := &fakeAPI{failing: map[string]int{}, calls: map[string]int{}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		step := "notes"
		switch body := string(data); {
		case strings.Contains(body, "You write flashcards"):
			step = "flashcards"
		case strings.Contains(body, "You write multiple-choice"):
			step = "quiz"
		}

		fake.mu.Lock()
		fake.calls[step]++
		status, failing := fake.failing[step]
		body := fake.body
		hold := fake.hold
		fake.mu.Unlock()
		if hold != nil {
			select {
			case <-hold:
			case <-r.Context().Done():
				return
			}
		}
		if failing {
			http.Error(w, body, status)
			return
		}
		wrapped, err := json.Marshal(map[string]any{
			"candidates": []any{map[string]any{
				"content":      map[string]any{"parts": []any{map[string]any{"text": answers[step]}}},
				"finishReason": "STOP",
			}},
			"usageMetadata": map[string]any{"promptTokenCount": 1000, "candidatesTokenCount": 100},
		})
		if err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, string(wrapped))
	}))
	t.Cleanup(server.Close)
	return &llm.Client{Key: "test-key", BaseURL: server.URL, HTTP: server.Client()}, fake
}
