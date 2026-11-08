// The worker makes notes. It runs until stopped, taking jobs from Postgres. Given
// -url it makes one note by hand instead, keeping its steps in local files.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"warden/internal/core"
	"warden/internal/jobs"
	"warden/internal/notes"
	"warden/internal/pipeline"
	"warden/internal/pipeline/captions"
	"warden/internal/pipeline/llm"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	link := flag.String("url", "", "make one note from this YouTube link, without the database")
	out := flag.String("out", "note.json", "with -url, file to write the note to")
	workDir := flag.String("work", ".work", "with -url, folder that keeps each lecture's finished steps")
	templateName := flag.String("template", notes.NEET, "with -url, the template to make it with: neet or general")
	flag.Parse()

	settings, err := core.Load(".env")
	if err != nil {
		return err
	}
	core.SetupLog(settings.LogLevel)
	captions.CookiesFile = settings.YTDLPCookies

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	gemini := llm.NewClient(settings.GeminiKey)
	if *link != "" {
		template, err := llm.TemplateFor(*templateName)
		if err != nil {
			return err
		}
		return makeOneNote(ctx, gemini, *link, template, *out, *workDir)
	}

	if err := settings.Require("DATABASE_URL", "GEMINI_API_KEY"); err != nil {
		return err
	}
	db, err := core.OpenDB(ctx, settings.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	slog.Info("worker started")
	jobs.New(db).Work(ctx, gemini)
	slog.Info("worker stopped")
	return nil
}

func makeOneNote(ctx context.Context, gemini *llm.Client, link string, template llm.Template, out, workDir string) error {
	note, err := pipeline.MakeNote(ctx, gemini, link, template, workDir)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(note, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("%s\n%d sections, %d flashcards, %d quiz questions, written to %s\n",
		note.Title, len(note.Sections), len(note.Flashcards), len(note.Quiz), out)
	if len(note.FailedSteps) > 0 {
		return fmt.Errorf("%s written without: %s. Run it again to retry only those", out, strings.Join(note.FailedSteps, ", "))
	}
	return nil
}
