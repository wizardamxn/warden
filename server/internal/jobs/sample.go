package jobs

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"warden/internal/notes"
	"warden/internal/pipeline"
	"warden/internal/pipeline/captions"
	"warden/internal/pipeline/llm"
)

// SaveFinished stores a note made elsewhere, such as the sample lecture's, as
// a finished job, so it's served like any other. It replaces what that job held.
// shortcut: the transcript isn't in a note, so the captions step is saved with no lines and a new prompt version can't remake this job, load the note again after one.
func (q *Queue) SaveFinished(ctx context.Context, videoID, template string, note notes.Note) error {
	t, err := llm.TemplateFor(template)
	if err != nil {
		return err
	}
	cards, quiz := note.Flashcards, note.Quiz
	note.Flashcards, note.Quiz, note.FailedSteps = nil, nil, nil
	results := map[string]any{
		"captions": struct {
			Video captions.Video
			Lines []captions.Line
		}{captions.Video{ID: videoID, Title: note.Title, Channel: note.Channel, DurationSeconds: note.DurationSeconds, Source: note.Source}, []captions.Line{}},
		"notes":      note,
		"flashcards": cards,
		"quiz":       quiz,
	}
	return pgx.BeginFunc(ctx, q.DB, func(tx pgx.Tx) error {
		if err := enqueue(ctx, tx, videoID, template); err != nil {
			return err
		}
		for _, step := range pipeline.Steps(t) {
			data, err := json.Marshal(results[step.Name])
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `
				UPDATE job_steps s SET status = 'done', result = $4::jsonb, prompt_version = $5, error = '', finished_at = now()
				FROM jobs j
				WHERE s.job_id = j.id AND j.video_id = $1 AND j.template = $2 AND s.name = $3`,
				videoID, template, step.Name, string(data), step.Version)
			if err != nil {
				return err
			}
		}
		return nil
	})
}
