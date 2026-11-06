package jobs

import (
	"context"
	"time"

	"warden/internal/core"
	"warden/internal/pipeline"
	"warden/internal/pipeline/llm"
)

// shortcut: polls every 2 seconds, use Postgres LISTEN/NOTIFY if the idle queries ever matter.
const pollEvery = 2 * time.Second

// Work runs jobs one at a time until ctx ends.
func (q *Queue) Work(ctx context.Context, gemini *llm.Client) {
	for ctx.Err() == nil {
		found, err := q.RunNext(ctx, gemini)
		if err != nil && ctx.Err() == nil {
			core.Log(ctx).Error("could not claim a job", "reason", err)
		}
		if found {
			continue
		}
		select {
		case <-time.After(pollEvery):
		case <-ctx.Done():
		}
	}
}

// RunNext claims one job and runs its unfinished steps. found is false if
// nothing was waiting.
func (q *Queue) RunNext(ctx context.Context, gemini *llm.Client) (found bool, err error) {
	job, takenBack, err := q.Claim(ctx)
	if job == nil || err != nil {
		return false, err
	}
	started := time.Now()
	ctx = core.LogWith(ctx, "video_id", job.VideoID, "job_id", job.ID)
	log := core.Log(ctx)
	log.Info("job claimed")
	for _, step := range takenBack {
		log.Warn("took back a step from a stopped worker", "step", step)
	}

	running, stop := context.WithCancel(ctx)
	defer stop()
	renewed := make(chan struct{})
	go func() {
		defer close(renewed)
		job.keepHold(running, stop)
	}()

	// Every step records its own outcome, so the error here adds nothing.
	_, _ = pipeline.Run(running, gemini, "https://www.youtube.com/watch?v="+job.VideoID, job)
	stop()
	<-renewed

	if err := job.Release(ctx); err != nil {
		log.Error("could not release the job", "reason", err)
	}
	if ctx.Err() != nil {
		log.Info("job stopped by shutdown, the next worker carries on", "duration", time.Since(started).Round(time.Millisecond).String())
		return true, nil
	}

	view, err := q.Get(ctx, job.VideoID)
	if err != nil {
		return true, err
	}
	logFinished := log.Info
	if view.Status == "partial" || view.Status == "failed" {
		logFinished = log.Warn
	}
	logFinished("job finished", "status", view.Status, "duration", time.Since(started).Round(time.Millisecond).String())
	return true, nil
}

// keepHold renews the hold while the job runs, so a slow job isn't mistaken for
// a stuck one. It stops the job if another worker took it over.
func (j *Job) keepHold(ctx context.Context, stop context.CancelFunc) {
	ticker := time.NewTicker(j.queue.StuckAfter / 4)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			held, err := j.renew(ctx)
			switch {
			case ctx.Err() != nil:
				return
			case err != nil:
				core.Log(ctx).Warn("could not renew the job's hold", "reason", err)
			case !held:
				core.Log(ctx).Error("lost the job to another worker, stopping")
				stop()
				return
			}
		}
	}
}
