// The sample program stores a finished note as the sample lecture's job, the
// note anyone can open before signing up. From server/:
//
//	go run ./cmd/sample -video k7Zb1iOV-ys -note ../apps/web/fixtures/k7Zb1iOV-ys.json
//
// Then set SAMPLE_VIDEO_ID to the same video ID for the API.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"warden/internal/core"
	"warden/internal/jobs"
	"warden/internal/notes"
	"warden/internal/pipeline/captions"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	videoID := flag.String("video", "", "the YouTube video ID of the note")
	notePath := flag.String("note", "", "a note.json file, as the worker's -url run writes")
	flag.Parse()

	id, err := captions.VideoID("https://youtu.be/" + *videoID)
	if err != nil {
		return errors.New("-video must be a YouTube video ID, such as k7Zb1iOV-ys")
	}
	data, err := os.ReadFile(*notePath)
	if err != nil {
		return fmt.Errorf("-note: %w", err)
	}
	var note notes.Note
	if err := json.Unmarshal(data, &note); err != nil {
		return fmt.Errorf("-note is not a note.json file: %w", err)
	}
	switch {
	case note.VideoID != id:
		return fmt.Errorf("the note is for video %q, not %q", note.VideoID, id)
	case len(note.Sections) == 0 || len(note.Flashcards) == 0 || len(note.Quiz) == 0:
		return errors.New("the note needs sections, flashcards and a quiz to be the sample")
	}

	settings, err := core.Load(".env")
	if err != nil {
		return err
	}
	if err := settings.Require("DATABASE_URL"); err != nil {
		return err
	}
	ctx := context.Background()
	db, err := core.OpenDB(ctx, settings.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := jobs.New(db).SaveFinished(ctx, id, notes.NEET, note); err != nil {
		return err
	}
	fmt.Printf("%s saved as the finished NEET note for %s. Set SAMPLE_VIDEO_ID=%s for the API.\n", note.Title, id, id)
	return nil
}
