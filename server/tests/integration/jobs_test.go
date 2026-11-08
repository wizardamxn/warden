// Integration tests run against a real Postgres, because claiming and saving
// depend on what Postgres does. They never call the real Gemini API or YouTube.
package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"warden/internal/jobs"
	"warden/internal/pipeline"
	"warden/internal/pipeline/llm"
)

// step finds one step of the pipeline by name.
func step(t *testing.T, name string) pipeline.Step {
	t.Helper()
	for _, s := range pipeline.Steps(llm.NEET) {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no step %q", name)
	return pipeline.Step{}
}

func runNext(t *testing.T, q *jobs.Queue, gemini *llm.Client) {
	t.Helper()
	found, err := q.RunNext(context.Background(), gemini)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected a job to run")
	}
}

func wantNothingToClaim(t *testing.T, q *jobs.Queue) {
	t.Helper()
	job, _, err := q.Claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if job != nil {
		t.Fatalf("job %d should not be claimable", job.ID)
	}
}

func claim(t *testing.T, q *jobs.Queue) *jobs.Job {
	t.Helper()
	job, _, err := q.Claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if job == nil {
		t.Fatal("expected a job to claim")
	}
	return job
}

func TestTwoWorkersCannotClaimTheSameJob(t *testing.T) {
	q := testQueue(t)
	const jobCount, workerCount = 5, 16
	for i := range jobCount {
		seedVideo(t, q, fmt.Sprintf("video%06d", i))
	}

	var mu sync.Mutex
	claimed := map[int64]int{}
	var workers sync.WaitGroup
	for range workerCount {
		workers.Go(func() {
			// Holding each job keeps it taken, as a busy worker would.
			for {
				job, _, err := q.Claim(context.Background())
				if err != nil {
					t.Error(err)
					return
				}
				if job == nil {
					return
				}
				mu.Lock()
				claimed[job.ID]++
				mu.Unlock()
			}
		})
	}
	workers.Wait()

	if len(claimed) != jobCount {
		t.Errorf("claimed %d jobs, want %d", len(claimed), jobCount)
	}
	for id, times := range claimed {
		if times != 1 {
			t.Errorf("job %d was claimed %d times", id, times)
		}
	}
}

func TestStepResultAndMarkerAreAtomic(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	seedVideo(t, q, testID)
	job := claim(t, q)

	if run, err := job.Start(ctx, step(t, "notes"), nil); !run || err != nil {
		t.Fatalf("run = %v, err = %v", run, err)
	}
	// Postgres refuses a NUL character in JSON, so this save fails part way.
	if err := job.Done(ctx, step(t, "notes"), map[string]string{"tldr": "bad\x00text"}, llm.Usage{}); err == nil {
		t.Fatal("expected the save to fail")
	}
	if got := stepOf(t, q, testID, "notes"); got.Status != "running" || got.HasResult {
		t.Errorf("a failed save must change nothing, got %+v", got)
	}

	_, err := q.DB.Exec(ctx, `UPDATE job_steps SET status = 'done', result = NULL WHERE name = 'notes'`)
	if err == nil {
		t.Error("the database must refuse a done marker with no result")
	}

	usage := llm.Usage{InputTokens: 1000, OutputTokens: 100}
	if err := job.Done(ctx, step(t, "notes"), map[string]string{"tldr": "good"}, usage); err != nil {
		t.Fatal(err)
	}
	if got := stepOf(t, q, testID, "notes"); got.Status != "done" || !got.HasResult || got.InputTokens != 1000 {
		t.Errorf("got %+v", got)
	}
	var cost float64
	if err := q.DB.QueryRow(ctx, `SELECT cost_usd FROM job_steps WHERE name = 'notes'`).Scan(&cost); err != nil {
		t.Fatal(err)
	}
	if want := usage.CostUSD(); cost != want {
		t.Errorf("cost = %v, want %v", cost, want)
	}
}

func TestCrashedStepIsReclaimableAfterTheTimeout(t *testing.T) {
	q := testQueue(t)
	q.StuckAfter = 300 * time.Millisecond
	ctx := context.Background()
	seedVideo(t, q, testID)

	// This worker dies mid-step: no release, no failure recorded.
	dead := claim(t, q)
	if _, err := dead.Start(ctx, step(t, "notes"), nil); err != nil {
		t.Fatal(err)
	}
	wantNothingToClaim(t, q)
	if got := stepOf(t, q, testID, "notes"); got.Status != "running" {
		t.Fatalf("got %+v", got)
	}

	time.Sleep(400 * time.Millisecond)
	job, takenBack, err := q.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if job == nil || len(takenBack) != 1 || takenBack[0] != "notes" {
		t.Fatalf("job = %+v, taken back = %v", job, takenBack)
	}
	if got := stepOf(t, q, testID, "notes"); got.Status != "pending" {
		t.Errorf("got %+v", got)
	}

	// The old worker wakes up late. Its result must not land.
	if err := dead.Done(ctx, step(t, "notes"), map[string]string{"tldr": "stale"}, llm.Usage{}); err == nil {
		t.Error("a worker that lost the step must not save over it")
	}
	if got := stepOf(t, q, testID, "notes"); got.HasResult {
		t.Errorf("got %+v", got)
	}

	if run, err := job.Start(ctx, step(t, "notes"), nil); !run || err != nil {
		t.Fatalf("run = %v, err = %v", run, err)
	}
	if got := stepOf(t, q, testID, "notes"); got.Attempts != 2 {
		t.Errorf("got %+v", got)
	}
}

func TestStepThatKeepsCrashingStops(t *testing.T) {
	q := testQueue(t)
	q.StuckAfter = 100 * time.Millisecond
	ctx := context.Background()
	seedVideo(t, q, testID)

	for range 3 {
		job := claim(t, q)
		if run, err := job.Start(ctx, step(t, "notes"), nil); !run || err != nil {
			t.Fatalf("run = %v, err = %v", run, err)
		}
		time.Sleep(120 * time.Millisecond)
	}
	claim(t, q)
	got := stepOf(t, q, testID, "notes")
	if got.Status != "failed" || got.Error == "" {
		t.Errorf("got %+v", got)
	}
}

func TestSlowJobKeepsItsHold(t *testing.T) {
	q := testQueue(t)
	q.StuckAfter = 200 * time.Millisecond
	seedVideo(t, q, testID)
	client, fake := fakeGemini(t)
	fake.hold = make(chan struct{})

	finished := make(chan error, 1)
	go func() {
		_, err := q.RunNext(context.Background(), client)
		finished <- err
	}()

	// Well past the timeout, the job is still held by the worker running it.
	time.Sleep(600 * time.Millisecond)
	wantNothingToClaim(t, q)
	view, err := q.Get(context.Background(), testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "running" {
		t.Errorf("a live job must be left alone, status = %q", view.Status)
	}
	close(fake.hold)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if got := stepOf(t, q, testID, "notes"); got.Status != "done" || got.Attempts != 1 {
		t.Errorf("got %+v", got)
	}
}

func TestJobWithNoWorkerLeftAlwaysEnds(t *testing.T) {
	const stuck = 100 * time.Millisecond
	tests := []struct {
		name  string
		start func(t *testing.T, q *jobs.Queue)
	}{
		{name: "worker died mid-step", start: func(t *testing.T, q *jobs.Queue) {
			if _, err := claim(t, q).Start(context.Background(), step(t, "notes"), nil); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "worker stopped cleanly", start: func(t *testing.T, q *jobs.Queue) {
			if err := claim(t, q).Release(context.Background()); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "no worker ever came", start: func(t *testing.T, q *jobs.Queue) {}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := testQueue(t)
			q.StuckAfter = stuck
			ctx := context.Background()
			seedVideo(t, q, testID)
			tt.start(t, q)

			// Only the API is left. Polling alone has to end the job.
			time.Sleep(stuck * 4)
			view, err := q.Get(ctx, testID, "neet")
			if err != nil {
				t.Fatal(err)
			}
			if view.Status != "failed" || view.Error == "" || strings.Contains(view.Error, "{") {
				t.Fatalf("view = %+v", view)
			}
			for _, s := range view.Steps {
				if s.Status == "pending" || s.Status == "running" {
					t.Errorf("step = %+v", s)
				}
			}

			// Try again starts it afresh, and the clock with it.
			view, err = q.Retry(ctx, testID, "neet")
			if err != nil {
				t.Fatal(err)
			}
			if view.Status != "queued" {
				t.Errorf("status after retry = %q", view.Status)
			}
		})
	}
}

func TestStuckStepGoesBackToFreeWhileOnlyTheAPIIsWatching(t *testing.T) {
	q := testQueue(t)
	q.StuckAfter = 200 * time.Millisecond
	ctx := context.Background()
	seedVideo(t, q, testID)
	if _, err := claim(t, q).Start(ctx, step(t, "notes"), nil); err != nil {
		t.Fatal(err)
	}

	time.Sleep(250 * time.Millisecond)
	view, err := q.Get(ctx, testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "queued" || view.Steps[1].Status != "pending" {
		t.Errorf("view = %+v", view)
	}
}

func TestStepPastItsAttemptsFailsWithoutAWorker(t *testing.T) {
	q := testQueue(t)
	q.StuckAfter = 200 * time.Millisecond
	ctx := context.Background()
	seedVideo(t, q, testID)
	if _, err := claim(t, q).Start(ctx, step(t, "notes"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := q.DB.Exec(ctx, `UPDATE job_steps SET attempts = 3 WHERE name = 'notes'`); err != nil {
		t.Fatal(err)
	}

	time.Sleep(250 * time.Millisecond)
	view, err := q.Get(ctx, testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Steps[1].Status != "failed" || view.Steps[1].Error == "" {
		t.Errorf("view = %+v", view)
	}
}

func TestRetryOnlyRunsFailedSteps(t *testing.T) {
	tests := []struct {
		name       string
		failing    string
		wantStatus string
		wantCalls  map[string]int
	}{
		{name: "flashcards", failing: "flashcards", wantStatus: "partial", wantCalls: map[string]int{"notes": 1, "flashcards": 2, "quiz": 1}},
		{name: "quiz", failing: "quiz", wantStatus: "partial", wantCalls: map[string]int{"notes": 1, "flashcards": 1, "quiz": 2}},
		{name: "notes", failing: "notes", wantStatus: "failed", wantCalls: map[string]int{"notes": 2, "flashcards": 1, "quiz": 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := testQueue(t)
			ctx := context.Background()
			seedVideo(t, q, testID)
			client, fake := fakeGemini(t)
			fake.fail(500, "failed", tt.failing)

			runNext(t, q, client)
			view, err := q.Get(ctx, testID, "neet")
			if err != nil {
				t.Fatal(err)
			}
			if view.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", view.Status, tt.wantStatus)
			}
			if tt.wantStatus == "failed" && (view.Note != nil || view.Error == "") {
				t.Errorf("view = %+v", view)
			}
			if tt.wantStatus == "partial" {
				if view.Note == nil || len(view.Note.FailedSteps) != 1 || view.Note.FailedSteps[0] != tt.failing {
					t.Errorf("note = %+v", view.Note)
				}
			}

			fake.fail(0, "")
			view, err = q.Retry(ctx, testID, "neet")
			if err != nil {
				t.Fatal(err)
			}
			if view.Status != "queued" {
				t.Errorf("status after retry = %q", view.Status)
			}
			runNext(t, q, client)
			view, err = q.Get(ctx, testID, "neet")
			if err != nil {
				t.Fatal(err)
			}
			if view.Status != "done" || view.Note == nil || len(view.Note.Flashcards) != 10 || len(view.Note.Quiz) != 5 || len(view.Note.FailedSteps) != 0 {
				t.Errorf("view = %+v", view)
			}
			fake.wantCalls(t, tt.wantCalls)
		})
	}
}

func TestReusedVideoMakesNoAICalls(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	seedVideo(t, q, testID)
	client, fake := fakeGemini(t)

	runNext(t, q, client)
	view, err := q.Enqueue(ctx, testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "done" || view.Note == nil {
		t.Fatalf("view = %+v", view)
	}
	wantNothingToClaim(t, q)
	fake.wantCalls(t, map[string]int{"notes": 1, "flashcards": 1, "quiz": 1})
}

func TestNewPromptVersionRunsOnlyThatStep(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	seedVideo(t, q, testID)
	client, fake := fakeGemini(t)
	runNext(t, q, client)

	if _, err := q.DB.Exec(ctx, `UPDATE job_steps SET prompt_version = 'older' WHERE name = 'quiz'`); err != nil {
		t.Fatal(err)
	}
	view, err := q.Enqueue(ctx, testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "queued" {
		t.Errorf("status = %q", view.Status)
	}
	runNext(t, q, client)
	fake.wantCalls(t, map[string]int{"notes": 1, "flashcards": 1, "quiz": 2})

	var version string
	if err := q.DB.QueryRow(ctx, `SELECT prompt_version FROM job_steps WHERE name = 'quiz'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != step(t, "quiz").Version {
		t.Errorf("version = %q", version)
	}
}

func TestNoteShowsBeforeTheOtherStepsFinish(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	seedVideo(t, q, testID)
	job := claim(t, q)

	if _, err := job.Start(ctx, step(t, "notes"), nil); err != nil {
		t.Fatal(err)
	}
	if err := job.Done(ctx, step(t, "notes"), map[string]string{"title": "Immunity"}, llm.Usage{}); err != nil {
		t.Fatal(err)
	}
	view, err := q.Get(ctx, testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "running" || view.Note == nil || view.Note.Title != "Immunity" {
		t.Fatalf("view = %+v", view)
	}
	if view.Note.Flashcards == nil || view.Note.Quiz == nil || view.Note.FailedSteps == nil {
		t.Errorf("arrays must be empty, not null: %+v", view.Note)
	}
}

func TestDailyQuotaFailsTheStepPlainlyWithOneCall(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	seedVideo(t, q, testID)
	client, fake := fakeGemini(t)
	fake.fail(429, `{"error":{"code":429,"details":[{"violations":[{"quotaId":"GenerateRequestsPerDayPerProjectPerModel-FreeTier"}]}]}}`, "notes")

	runNext(t, q, client)
	view, err := q.Get(ctx, testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "failed" || !strings.Contains(view.Error, "tomorrow") || strings.Contains(view.Error, "{") {
		t.Errorf("view = %+v", view)
	}
	fake.wantCalls(t, map[string]int{"notes": 1, "flashcards": 1, "quiz": 1})
}

func TestShutdownFreesTheStepAndANewWorkerCarriesOn(t *testing.T) {
	q := testQueue(t)
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	seedVideo(t, q, testID)
	client, fake := fakeGemini(t)

	// The notes step finishes, then Ctrl+C arrives in the middle of flashcards.
	runDone := make(chan error, 1)
	fake.mu.Lock()
	fake.hold = make(chan struct{})
	fake.mu.Unlock()
	go func() {
		_, err := q.RunNext(ctx, client)
		runDone <- err
	}()
	waitFor(t, func() bool { return fake.counts()["notes"] == 1 })
	fake.mu.Lock()
	hold := fake.hold
	fake.hold = make(chan struct{}) // flashcards waits on this one
	fake.mu.Unlock()
	close(hold)
	waitFor(t, func() bool { return fake.counts()["flashcards"] == 1 })
	stop()
	if err := <-runDone; err != nil {
		t.Fatal(err)
	}

	if got := stepOf(t, q, testID, "notes"); got.Status != "done" {
		t.Errorf("notes = %+v", got)
	}
	if got := stepOf(t, q, testID, "flashcards"); got.Status != "pending" || got.Error != "" {
		t.Errorf("flashcards = %+v", got)
	}

	// No waiting for the timeout: a clean stop lets go of the job.
	fake.mu.Lock()
	fake.hold = nil
	fake.mu.Unlock()
	runNext(t, q, client)
	fake.wantCalls(t, map[string]int{"notes": 1, "flashcards": 2, "quiz": 1})
	view, err := q.Get(context.Background(), testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "done" {
		t.Errorf("status = %q", view.Status)
	}
}

func TestSaveFinishesEvenDuringShutdown(t *testing.T) {
	q := testQueue(t)
	seedVideo(t, q, testID)
	ctx, stop := context.WithCancel(context.Background())
	job := claim(t, q)
	if _, err := job.Start(ctx, step(t, "notes"), nil); err != nil {
		t.Fatal(err)
	}
	stop()
	// Gemini answered just as Ctrl+C arrived. That answer is paid for, so it's kept.
	if err := job.Done(ctx, step(t, "notes"), map[string]string{"tldr": "paid for"}, llm.Usage{}); err != nil {
		t.Fatal(err)
	}
	if got := stepOf(t, q, testID, "notes"); got.Status != "done" {
		t.Errorf("got %+v", got)
	}
}

func TestCaptionsFailureFailsEveryStepAndRetriesThemAll(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	if _, err := q.Enqueue(ctx, testID, "neet"); err != nil {
		t.Fatal(err)
	}
	client, fake := fakeGemini(t)
	t.Setenv("PATH", "") // yt-dlp can't start, so nothing reaches YouTube

	runNext(t, q, client)
	view, err := q.Get(ctx, testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "failed" || view.Note != nil || strings.Contains(view.Error, "yt-dlp") {
		t.Errorf("view = %+v", view)
	}
	for _, s := range view.Steps {
		if s.Status != "failed" || s.Error == "" {
			t.Errorf("step = %+v", s)
		}
	}
	fake.wantCalls(t, map[string]int{})

	view, err = q.Retry(ctx, testID, "neet")
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "queued" {
		t.Errorf("status = %q", view.Status)
	}
}

func TestUnknownVideo(t *testing.T) {
	q := testQueue(t)
	ctx := context.Background()
	if _, err := q.Get(ctx, testID, "neet"); !errors.Is(err, jobs.ErrNotFound) {
		t.Errorf("get: %v", err)
	}
	if _, err := q.Retry(ctx, testID, "neet"); !errors.Is(err, jobs.ErrNotFound) {
		t.Errorf("retry: %v", err)
	}
}

func TestMigrationsRunOnce(t *testing.T) {
	q := testQueue(t)
	var applied int
	if err := q.DB.QueryRow(context.Background(), `SELECT count(*) FROM schema_migrations`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied == 0 {
		t.Error("no migrations recorded")
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
