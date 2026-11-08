// Package pipeline runs the steps that turn a link into a note, in order.
// Each finished step is saved, so a re-run only does what's missing and never
// pays for the same AI work twice.
package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"warden/internal/core"
	"warden/internal/notes"
	"warden/internal/pipeline/captions"
	"warden/internal/pipeline/llm"
)

// CaptionsVersion plays the prompt version's part for the captions step.
const CaptionsVersion = "v1"

// The transcript is the same whatever the template, so one-off runs share it.
const transcriptFile = "transcript.json"

// Step is one step of the pipeline. A saved result is reused only while its
// Version stays the same.
type Step struct {
	Name    string
	Version string
	file    string // where a one-off run keeps it
}

// StepNames lists every step in the order they run.
var StepNames = []string{"captions", "notes", "flashcards", "quiz"}

// Steps lists a template's steps in the order they run, with its prompt versions.
func Steps(t llm.Template) []Step {
	return []Step{
		{"captions", CaptionsVersion, transcriptFile},
		{"notes", t.NoteVersion, "notes-" + t.NoteVersion + ".json"},
		{"flashcards", t.FlashcardVersion, "flashcards-" + t.FlashcardVersion + ".json"},
		{"quiz", t.QuizVersion, "quiz-" + t.QuizVersion + ".json"},
	}
}

// Store keeps each step's result: local files for a one-off run, Postgres for
// the worker's jobs. Both run the same pipeline.
type Store interface {
	// Start says whether the step should run. If not, a saved result is read into into.
	Start(ctx context.Context, step Step, into any) (run bool, err error)
	// Done saves a finished step's result.
	Done(ctx context.Context, step Step, result any, usage llm.Usage) error
	// Fail records that a started step didn't finish.
	Fail(ctx context.Context, step Step, reason error, usage llm.Usage) error
}

// errNoTranscript stops the AI steps when the captions step has nothing for them.
var errNoTranscript = errors.New("no transcript")

// transcript is the saved result of the captions step.
type transcript struct {
	Video captions.Video
	Lines []captions.Line
}

// MakeNote is the one-off run: it makes the note for one link, keeping each
// lecture's finished steps in workDir.
func MakeNote(ctx context.Context, gemini *llm.Client, link string, t llm.Template, workDir string) (notes.Note, error) {
	id, err := captions.VideoID(link)
	if err != nil {
		return notes.Note{}, err
	}
	dir := filepath.Join(workDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return notes.Note{}, err
	}
	return Run(core.LogWith(ctx, "video_id", id), gemini, link, t, fileStore{dir})
}

// Run does every step that still needs doing. The three AI steps don't depend
// on each other, so all of them run even if one fails. Without notes there's
// nothing to show, so that's an error. A failed flashcards or quiz step comes
// back in the note's FailedSteps instead.
func Run(ctx context.Context, gemini *llm.Client, link string, tmpl llm.Template, store Store) (notes.Note, error) {
	steps := Steps(tmpl)
	captionsStep, notesStep, flashcardsStep, quizStep := steps[0], steps[1], steps[2], steps[3]
	t, transcriptErr := transcriptStep(ctx, link, captionsStep, store)

	note, notesErr := runStep(ctx, store, notesStep, func(ctx context.Context) (notes.Note, llm.Usage, error) {
		if transcriptErr != nil {
			return notes.Note{}, llm.Usage{}, transcriptErr
		}
		return llm.MakeNote(ctx, gemini, tmpl, t.Video, t.Lines)
	})
	cards, cardsErr := runStep(ctx, store, flashcardsStep, func(ctx context.Context) ([]notes.Flashcard, llm.Usage, error) {
		if transcriptErr != nil {
			return nil, llm.Usage{}, transcriptErr
		}
		return llm.MakeFlashcards(ctx, gemini, tmpl, t.Video, t.Lines)
	})
	quiz, quizErr := runStep(ctx, store, quizStep, func(ctx context.Context) ([]notes.Question, llm.Usage, error) {
		if transcriptErr != nil {
			return nil, llm.Usage{}, transcriptErr
		}
		return llm.MakeQuiz(ctx, gemini, tmpl, t.Video, t.Lines)
	})
	if notesErr != nil {
		return notes.Note{}, notesErr
	}

	var failed []string
	if cardsErr != nil {
		failed = append(failed, flashcardsStep.Name)
	}
	if quizErr != nil {
		failed = append(failed, quizStep.Name)
	}
	return Assemble(note, tmpl.Name, cards, quiz, failed), nil
}

// Assemble puts the separately saved parts together as one note. The template
// comes from the job, never from the AI's answer.
func Assemble(note notes.Note, template string, cards []notes.Flashcard, quiz []notes.Question, failed []string) notes.Note {
	note.Template = template
	note.KeyPoints = orEmpty(note.KeyPoints)
	note.Sections = orEmpty(note.Sections)
	for i := range note.Sections {
		note.Sections[i].Bullets = orEmpty(note.Sections[i].Bullets)
	}
	note.Mnemonics = orEmpty(note.Mnemonics)
	note.Glossary = orEmpty(note.Glossary)
	note.Flashcards = orEmpty(cards)
	note.Quiz = orEmpty(quiz)
	for i := range note.Quiz {
		note.Quiz[i].Options = orEmpty(note.Quiz[i].Options)
	}
	note.FailedSteps = orEmpty(failed)
	return note
}

// orEmpty keeps arrays out of the JSON as [] rather than null.
func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

func transcriptStep(ctx context.Context, link string, captionsStep Step, store Store) (transcript, error) {
	t, err := runStep(ctx, store, captionsStep, func(ctx context.Context) (transcript, llm.Usage, error) {
		tempDir, err := os.MkdirTemp("", "warden-")
		if err != nil {
			return transcript{}, llm.Usage{}, err
		}
		defer os.RemoveAll(tempDir)

		video, rawLines, err := captions.Fetch(ctx, link, tempDir)
		if err != nil {
			return transcript{}, llm.Usage{}, err
		}
		lines := captions.Clean(rawLines)
		if len(lines) == 0 {
			return transcript{}, llm.Usage{}, captions.ErrNoCaptions
		}
		return transcript{Video: video, Lines: lines}, llm.Usage{}, nil
	})
	if err != nil {
		return t, fmt.Errorf("%w: %w", errNoTranscript, err)
	}
	if len(t.Lines) == 0 {
		return t, errNoTranscript
	}
	return t, nil
}

// runStep runs one step, unless the store has it saved or wasn't asked for it.
func runStep[T any](ctx context.Context, store Store, step Step, generate func(ctx context.Context) (T, llm.Usage, error)) (T, error) {
	ctx = core.LogWith(ctx, "step", step.Name)
	log := core.Log(ctx)

	var saved T
	run, err := store.Start(ctx, step, &saved)
	if err != nil {
		if ctx.Err() == nil {
			log.Error("could not start the step", "reason", err)
		}
		return saved, err
	}
	if !run {
		log.Debug("step not needed")
		return saved, nil
	}

	log.Info("step started")
	started := time.Now()
	result, usage, err := generate(ctx)
	if err == nil {
		err = store.Done(ctx, step, result, usage)
	}
	fields := []any{"duration", time.Since(started).Round(time.Millisecond).String(),
		"input_tokens", usage.InputTokens, "output_tokens", usage.OutputTokens,
		"cost_usd", usage.CostUSD(), "cost_inr", usage.CostRupees()}
	if err == nil {
		log.Info("step finished", fields...)
		return result, nil
	}

	if ctx.Err() != nil {
		log.Info("step stopped by shutdown", fields...)
	} else {
		log.Error("step failed", append(fields, "reason", err)...)
	}
	if failErr := store.Fail(ctx, step, err, usage); failErr != nil {
		log.Error("could not record the step's failure", "reason", failErr)
	}
	return result, err
}

// fileStore keeps a lecture's steps as files in one folder. A failed step
// leaves nothing behind, so the next run tries it again.
type fileStore struct{ dir string }

func (f fileStore) Start(_ context.Context, step Step, into any) (bool, error) {
	found, err := load(filepath.Join(f.dir, step.file), into)
	return !found, err
}

func (f fileStore) Done(_ context.Context, step Step, result any, _ llm.Usage) error {
	return save(filepath.Join(f.dir, step.file), result)
}

func (f fileStore) Fail(context.Context, Step, error, llm.Usage) error { return nil }

// load reads a saved step. found is false if the step hasn't finished yet.
func load(path string, into any) (found bool, err error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(data, into)
}

// save writes to a temporary file first, so a crash can't leave a half-written
// file that looks like a finished step.
func save(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
