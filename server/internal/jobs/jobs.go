// Package jobs is the job list in Postgres. The API adds jobs and reads them,
// the worker claims them. The two programs meet only here.
package jobs

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"warden/internal/notes"
	"warden/internal/pipeline"
	"warden/internal/pipeline/llm"
)

const (
	// A job with no sign of life for this long goes back to free.
	stuckAfter = 10 * time.Minute
	// A step that keeps killing its worker stops here instead of spending AI calls forever.
	maxAttempts = 3
	// With no worker to pick it up, a job ends after this many stuckAfters, because
	// the website polls until a job ends. Try again gives it a new start.
	giveUpAfter = 3

	saveTimeout = 10 * time.Second
)

var (
	ErrNotFound = errors.New("no job for this video")
	errLost     = errors.New("this step now belongs to another worker")
)

// Queue is the job list.
type Queue struct {
	DB         *pgxpool.Pool
	StuckAfter time.Duration
}

// New makes a queue on an open database.
func New(db *pgxpool.Pool) *Queue {
	return &Queue{DB: db, StuckAfter: stuckAfter}
}

// StepView is one step as the website sees it.
type StepView struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error"`
}

// View is a job as the website sees it.
type View struct {
	VideoID string      `json:"video_id"`
	Status  string      `json:"status"`
	Steps   []StepView  `json:"steps"`
	Note    *notes.Note `json:"note"`
	Error   string      `json:"error"`
}

// Finished says no more work is coming unless someone retries.
func (v View) Finished() bool {
	return v.Status == "done" || v.Status == "partial" || v.Status == "failed"
}

// Enqueue makes sure a video has a job under a template and returns it. A step
// already saved with today's prompt is left alone, so a repeat request adds no
// work and no AI calls.
func (q *Queue) Enqueue(ctx context.Context, videoID, template string) (View, error) {
	err := pgx.BeginFunc(ctx, q.DB, func(tx pgx.Tx) error {
		return enqueue(ctx, tx, videoID, template)
	})
	if err != nil {
		return View{}, err
	}
	return q.Get(ctx, videoID, template)
}

// enqueue is Enqueue inside the caller's transaction.
// shortcut: a new prompt version overwrites the old result, keep old rows if we ever want to compare versions.
func enqueue(ctx context.Context, tx pgx.Tx, videoID, template string) error {
	t, err := llm.TemplateFor(template)
	if err != nil {
		return err
	}
	var names, versions []string
	for _, step := range pipeline.Steps(t) {
		names = append(names, step.Name)
		versions = append(versions, step.Version)
	}
	_, err = tx.Exec(ctx, `INSERT INTO jobs (video_id, template) VALUES ($1, $2) ON CONFLICT (video_id, template) DO NOTHING`, videoID, template)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO job_steps (job_id, name, prompt_version)
		SELECT j.id, s.name, s.version
		FROM jobs j, unnest($3::text[], $4::text[]) AS s (name, version)
		WHERE j.video_id = $1 AND j.template = $2
		ON CONFLICT (job_id, name) DO UPDATE SET
			prompt_version = EXCLUDED.prompt_version,
			status = 'pending', result = NULL, error = '', started_at = NULL, finished_at = NULL
		WHERE job_steps.prompt_version <> EXCLUDED.prompt_version`, videoID, template, names, versions)
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE jobs SET active_at = now() WHERE video_id = $1 AND template = $2`, videoID, template)
	return err
}

// Retry frees a job's failed steps and nothing else.
func (q *Queue) Retry(ctx context.Context, videoID, template string) (View, error) {
	err := pgx.BeginFunc(ctx, q.DB, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE job_steps s SET status = 'pending', error = '', finished_at = NULL
			FROM jobs j
			WHERE s.job_id = j.id AND j.video_id = $1 AND j.template = $2 AND s.status = 'failed'`, videoID, template)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE jobs SET active_at = now() WHERE video_id = $1 AND template = $2`, videoID, template)
		return err
	})
	if err != nil {
		return View{}, err
	}
	return q.Get(ctx, videoID, template)
}

// stepRow is one step as stored. Result is nil for captions: the transcript is
// big, and the website doesn't need it.
type stepRow struct {
	Name   string
	Status string
	Error  string
	Result []byte
}

// Get reads a job, with as much of the note as is ready. The website keeps
// asking until a job ends, so this is also what ends a job with no worker left:
// the rule can't depend on a worker that may never come.
func (q *Queue) Get(ctx context.Context, videoID, template string) (View, error) {
	view, jobID, stalled, err := q.read(ctx, videoID, template)
	if err != nil || !stalled {
		return view, err
	}
	if err := q.settle(ctx, jobID); err != nil {
		return View{}, err
	}
	view, _, _, err = q.read(ctx, videoID, template)
	return view, err
}

// read loads a job. stalled says nobody holds it and it needs settling.
func (q *Queue) read(ctx context.Context, videoID, template string) (view View, jobID int64, stalled bool, err error) {
	var held, idle bool
	err = q.DB.QueryRow(ctx, `
		SELECT id, COALESCE(locked_until > now(), false), active_at < now() - make_interval(secs => $3)
		FROM jobs WHERE video_id = $1 AND template = $2`, videoID, template, q.giveUpSeconds()).Scan(&jobID, &held, &idle)
	if errors.Is(err, pgx.ErrNoRows) {
		return View{}, 0, false, ErrNotFound
	}
	if err != nil {
		return View{}, 0, false, err
	}

	rows, err := q.DB.Query(ctx, `
		SELECT name, status, error, CASE WHEN name = 'captions' THEN NULL ELSE result END
		FROM job_steps WHERE job_id = $1`, jobID)
	if err != nil {
		return View{}, 0, false, err
	}
	stored, err := pgx.CollectRows(rows, pgx.RowToStructByPos[stepRow])
	if err != nil {
		return View{}, 0, false, err
	}
	view, err = buildView(videoID, template, held, stored)
	if err != nil {
		return View{}, 0, false, err
	}

	for _, row := range stored {
		if !held && (row.Status == "running" || row.Status == "pending" && idle) {
			stalled = true
		}
	}
	return view, jobID, stalled, nil
}

func (q *Queue) giveUpSeconds() float64 {
	return (q.StuckAfter * giveUpAfter).Seconds()
}

// settle moves a job no worker holds: stuck steps go back to free or stop at
// their last attempt, and a job idle for too long ends. A worker claiming the
// job at this moment wins, and this does nothing.
func (q *Queue) settle(ctx context.Context, jobID int64) error {
	return pgx.BeginFunc(ctx, q.DB, func(tx pgx.Tx) error {
		var idle bool
		err := tx.QueryRow(ctx, `
			SELECT active_at < now() - make_interval(secs => $2) FROM jobs
			WHERE id = $1 AND (locked_until IS NULL OR locked_until < now())
			FOR UPDATE SKIP LOCKED`, jobID, q.giveUpSeconds()).Scan(&idle)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if idle {
			_, err = tx.Exec(ctx, `
				UPDATE job_steps SET status = 'failed', error = $2, finished_at = now()
				WHERE job_id = $1 AND status IN ('pending', 'running')`,
				jobID, "We couldn't get to this lecture in time. Please try again.")
			return err
		}
		_, err = takeBack(ctx, tx, jobID)
		return err
	})
}

// takeBack frees steps left "running" by a worker that died, unless they've had
// their last attempt. The caller must hold the job's row.
func takeBack(ctx context.Context, tx pgx.Tx, jobID int64) ([]string, error) {
	rows, err := tx.Query(ctx, `
		UPDATE job_steps SET
			status = CASE WHEN attempts >= $2 THEN 'failed' ELSE 'pending' END,
			error = CASE WHEN attempts >= $2 THEN $3 ELSE '' END,
			finished_at = CASE WHEN attempts >= $2 THEN now() END
		WHERE job_id = $1 AND status = 'running'
		RETURNING name`, jobID, maxAttempts, "We couldn't finish this step. Please try again.")
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// buildView works out the status from the steps, so there's only one place
// that says how a job is doing.
func buildView(videoID, template string, held bool, stored []stepRow) (View, error) {
	byName := map[string]stepRow{}
	for _, row := range stored {
		byName[row.Name] = row
	}

	view := View{VideoID: videoID}
	for _, name := range pipeline.StepNames {
		row := byName[name]
		view.Steps = append(view.Steps, StepView{Name: name, Status: row.Status, Error: row.Error})
	}
	view.Status = overallStatus(view.Steps, held)
	if view.Status == "failed" {
		view.Error = firstError(view.Steps)
	}

	if byName["notes"].Status != "done" {
		return view, nil
	}
	var note notes.Note
	if err := json.Unmarshal(byName["notes"].Result, &note); err != nil {
		return View{}, err
	}
	var cards []notes.Flashcard
	if row := byName["flashcards"]; row.Status == "done" {
		if err := json.Unmarshal(row.Result, &cards); err != nil {
			return View{}, err
		}
	}
	var quiz []notes.Question
	if row := byName["quiz"]; row.Status == "done" {
		if err := json.Unmarshal(row.Result, &quiz); err != nil {
			return View{}, err
		}
	}
	var failed []string
	for _, name := range []string{"flashcards", "quiz"} {
		if byName[name].Status == "failed" {
			failed = append(failed, name)
		}
	}
	assembled := pipeline.Assemble(note, template, cards, quiz, failed)
	view.Note = &assembled
	return view, nil
}

func overallStatus(steps []StepView, held bool) string {
	count := map[string]int{}
	notesDone := false
	for _, step := range steps {
		count[step.Status]++
		if step.Name == "notes" && step.Status == "done" {
			notesDone = true
		}
	}
	switch {
	case count["running"] > 0, count["pending"] > 0 && held:
		return "running"
	case count["pending"] > 0:
		return "queued"
	case !notesDone:
		return "failed"
	case count["failed"] > 0:
		return "partial"
	}
	return "done"
}

func firstError(steps []StepView) string {
	for _, step := range steps {
		if step.Error != "" {
			return step.Error
		}
	}
	return ""
}

// Job is a job one worker holds until it's released or the hold runs out.
// It's the pipeline's Store for that job.
type Job struct {
	ID       int64
	VideoID  string
	Template string
	queue    *Queue
	owner    string         // proves the hold on the job is still ours
	attempts map[string]int // proves each running step is still ours
}

// Claim takes the next job with work to do, or returns nil if there is none.
// SKIP LOCKED lets workers pass each other instead of waiting or sharing.
func (q *Queue) Claim(ctx context.Context) (*Job, []string, error) {
	job := &Job{queue: q, owner: rand.Text(), attempts: map[string]int{}}
	var takenBack []string
	err := pgx.BeginFunc(ctx, q.DB, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT j.id, j.video_id, j.template FROM jobs j
			WHERE (j.locked_until IS NULL OR j.locked_until < now())
				AND EXISTS (SELECT 1 FROM job_steps s WHERE s.job_id = j.id AND s.status IN ('pending', 'running'))
			ORDER BY j.id
			LIMIT 1
			FOR UPDATE OF j SKIP LOCKED`).Scan(&job.ID, &job.VideoID, &job.Template)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE jobs SET locked_by = $2, locked_until = now() + make_interval(secs => $3), active_at = now() WHERE id = $1`,
			job.ID, job.owner, q.StuckAfter.Seconds())
		if err != nil {
			return err
		}
		takenBack, err = takeBack(ctx, tx, job.ID)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return job, takenBack, nil
}

// renew pushes the hold's end further out. held is false if the job has been
// taken back from this worker.
func (j *Job) renew(ctx context.Context) (held bool, err error) {
	tag, err := j.queue.DB.Exec(ctx, `
		UPDATE jobs SET locked_until = now() + make_interval(secs => $3), active_at = now() WHERE id = $1 AND locked_by = $2`,
		j.ID, j.owner, j.queue.StuckAfter.Seconds())
	return tag.RowsAffected() > 0, err
}

// Release lets the job go, so any new work on it can be claimed straight away.
func (j *Job) Release(ctx context.Context) error {
	ctx, cancel := saving(ctx)
	defer cancel()
	_, err := j.queue.DB.Exec(ctx, `UPDATE jobs SET locked_by = NULL, locked_until = NULL, active_at = now() WHERE id = $1 AND locked_by = $2`, j.ID, j.owner)
	return err
}

// saving keeps a save going through shutdown: the work it records is already
// done, and for an AI step, already paid for.
func saving(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), saveTimeout)
}

// Start marks a free step as running. A step that isn't free doesn't run: if
// it's done its result comes back, and if it failed it waits for a retry.
func (j *Job) Start(ctx context.Context, step pipeline.Step, into any) (bool, error) {
	var attempts int
	err := j.queue.DB.QueryRow(ctx, `
		UPDATE job_steps SET status = 'running', attempts = attempts + 1, prompt_version = $3,
			started_at = now(), finished_at = NULL, error = ''
		WHERE job_id = $1 AND name = $2 AND status = 'pending'
		RETURNING attempts`, j.ID, step.Name, step.Version).Scan(&attempts)
	if err == nil {
		j.attempts[step.Name] = attempts
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}

	var saved []byte
	err = j.queue.DB.QueryRow(ctx, `SELECT result FROM job_steps WHERE job_id = $1 AND name = $2 AND status = 'done'`, j.ID, step.Name).Scan(&saved)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return false, json.Unmarshal(saved, into)
}

// Done saves a step's result and its "done" marker in one update, so one can
// never exist without the other. A worker that lost the step saves nothing.
func (j *Job) Done(ctx context.Context, step pipeline.Step, result any, usage llm.Usage) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	ctx, cancel := saving(ctx)
	defer cancel()
	tag, err := j.queue.DB.Exec(ctx, `
		UPDATE job_steps SET status = 'done', result = $3::jsonb, error = '', finished_at = now(),
			input_tokens = input_tokens + $4, output_tokens = output_tokens + $5, cost_usd = cost_usd + $6
		WHERE job_id = $1 AND name = $2 AND status = 'running' AND attempts = $7`,
		// As text, because Supabase's pooler mode would send bytes as bytea.
		j.ID, step.Name, string(data), usage.InputTokens, usage.OutputTokens, usage.CostUSD(), j.attempts[step.Name])
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errLost
	}
	return nil
}

// Fail records a plain reason for students. A step cut off by shutdown goes back
// to free instead, because nothing is wrong with it.
func (j *Job) Fail(ctx context.Context, step pipeline.Step, reason error, usage llm.Usage) error {
	status, message := "failed", pipeline.PlainError(step.Name, reason)
	if ctx.Err() != nil {
		status, message = "pending", ""
	}
	ctx, cancel := saving(ctx)
	defer cancel()
	_, err := j.queue.DB.Exec(ctx, `
		UPDATE job_steps SET status = $3, error = $4,
			finished_at = CASE WHEN $3 = 'failed' THEN now() END,
			input_tokens = input_tokens + $5, output_tokens = output_tokens + $6, cost_usd = cost_usd + $7
		WHERE job_id = $1 AND name = $2 AND status = 'running' AND attempts = $8`,
		j.ID, step.Name, status, message, usage.InputTokens, usage.OutputTokens, usage.CostUSD(), j.attempts[step.Name])
	return err
}
