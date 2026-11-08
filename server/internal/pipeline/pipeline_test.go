package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"warden/internal/notes"
	"warden/internal/pipeline/captions"
	"warden/internal/pipeline/llm"
)

const (
	testLink = "https://youtu.be/VjnHhmST8GY"
	testID   = "VjnHhmST8GY"
)

// The files a one-off NEET run keeps its steps in.
var (
	notesFile      = Steps(llm.NEET)[1].file
	flashcardsFile = Steps(llm.NEET)[2].file
	quizFile       = Steps(llm.NEET)[3].file
)

func TestNEETFilesKeepTheirNames(t *testing.T) {
	// Lectures made before templates are kept under these names in .work/.
	if notesFile != "notes-v4.json" || flashcardsFile != "flashcards-v2.json" || quizFile != "quiz-v3.json" {
		t.Errorf("files = %s, %s, %s", notesFile, flashcardsFile, quizFile)
	}
}

func TestTemplatesKeepTheirStepsApart(t *testing.T) {
	neet, general := Steps(llm.NEET), Steps(llm.General)
	if neet[0].file != general[0].file {
		t.Error("both templates should share the transcript")
	}
	for i := 1; i < len(neet); i++ {
		if neet[i].file == general[i].file || neet[i].Version == general[i].Version {
			t.Errorf("%s: NEET and General share %s / %s", neet[i].Name, neet[i].file, neet[i].Version)
		}
	}
}

// fakeAPI answers each step in its own way. Steps named in failing get an
// error, and every call is counted by step.
type fakeAPI struct {
	mu      sync.Mutex
	failing map[string]bool
	calls   map[string]int
}

func (f *fakeAPI) setFailing(steps ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing = map[string]bool{}
	for _, step := range steps {
		f.failing[step] = true
	}
}

func (f *fakeAPI) total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	sum := 0
	for _, calls := range f.calls {
		sum += calls
	}
	return sum
}

func (f *fakeAPI) step(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[name]
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

func stepOf(requestBody string) string {
	switch {
	case strings.Contains(requestBody, "You write flashcards"):
		return "flashcards"
	case strings.Contains(requestBody, "You write multiple-choice"):
		return "quiz"
	}
	return "notes"
}

func fakeGemini(t *testing.T) (*llm.Client, *fakeAPI) {
	t.Helper()
	answers := fakeAnswers(t)
	fake := &fakeAPI{failing: map[string]bool{}, calls: map[string]int{}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		step := stepOf(string(body))
		fake.mu.Lock()
		fake.calls[step]++
		failing := fake.failing[step]
		fake.mu.Unlock()
		if failing {
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		wrapped, err := json.Marshal(map[string]any{
			"candidates": []any{map[string]any{
				"content":      map[string]any{"parts": []any{map[string]any{"text": answers[step]}}},
				"finishReason": "STOP",
			}},
		})
		if err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, string(wrapped))
	}))
	t.Cleanup(server.Close)
	return &llm.Client{Key: "test-key", BaseURL: server.URL, HTTP: server.Client()}, fake
}

// workDirWithTranscript fakes a finished captions step, so tests never run yt-dlp.
func workDirWithTranscript(t *testing.T) string {
	t.Helper()
	workDir := t.TempDir()
	dir := filepath.Join(workDir, testID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	saved := transcript{
		Video: captions.Video{ID: testID, Title: "Immunity", DurationSeconds: 100, Source: "auto-captions"},
		Lines: []captions.Line{{StartSeconds: 0, Text: "immunity"}},
	}
	if err := save(filepath.Join(dir, transcriptFile), saved); err != nil {
		t.Fatal(err)
	}
	return workDir
}

func stepSaved(t *testing.T, workDir, file string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(workDir, testID, file))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

func TestFinishedStepsAreNotRepeated(t *testing.T) {
	client, fake := fakeGemini(t)
	workDir := workDirWithTranscript(t)

	first, err := MakeNote(context.Background(), client, testLink, llm.NEET, workDir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MakeNote(context.Background(), client, testLink, llm.NEET, workDir)
	if err != nil {
		t.Fatal(err)
	}
	if fake.total() != 3 {
		t.Errorf("gemini calls = %d, want 3", fake.total())
	}
	if first.TLDR == "" || first.TLDR != second.TLDR || second.Title != "Immunity" || second.VideoID != testID || second.Template != "neet" {
		t.Errorf("first = %+v, second = %+v", first, second)
	}
	if len(second.Flashcards) != 10 || len(second.Quiz) != 5 || len(second.FailedSteps) != 0 {
		t.Errorf("second = %+v", second)
	}
}

func TestFailedStepRunsAgainWithoutLosingTheTranscript(t *testing.T) {
	client, fake := fakeGemini(t)
	fake.setFailing("notes", "flashcards", "quiz")
	workDir := workDirWithTranscript(t)

	if _, err := MakeNote(context.Background(), client, testLink, llm.NEET, workDir); err == nil {
		t.Fatal("expected the notes step to fail")
	}
	if !stepSaved(t, workDir, transcriptFile) {
		t.Error("transcript should survive a failed notes step")
	}
	if stepSaved(t, workDir, notesFile) {
		t.Error("a failed step must not look finished")
	}

	fake.setFailing()
	if _, err := MakeNote(context.Background(), client, testLink, llm.NEET, workDir); err != nil {
		t.Fatal(err)
	}
	if fake.total() != 6 {
		t.Errorf("gemini calls = %d, want 6", fake.total())
	}
}

func TestStudyStepFailureKeepsTheRest(t *testing.T) {
	tests := []struct {
		name       string
		failing    string
		wantCards  int
		wantQuiz   int
		failedFile string
		savedFile  string
	}{
		{name: "flashcards", failing: "flashcards", wantCards: 0, wantQuiz: 5, failedFile: flashcardsFile, savedFile: quizFile},
		{name: "quiz", failing: "quiz", wantCards: 10, wantQuiz: 0, failedFile: quizFile, savedFile: flashcardsFile},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, fake := fakeGemini(t)
			fake.setFailing(tt.failing)
			workDir := workDirWithTranscript(t)

			note, err := MakeNote(context.Background(), client, testLink, llm.NEET, workDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(note.Flashcards) != tt.wantCards || len(note.Quiz) != tt.wantQuiz {
				t.Errorf("flashcards = %d, quiz = %d", len(note.Flashcards), len(note.Quiz))
			}
			if len(note.FailedSteps) != 1 || note.FailedSteps[0] != tt.failing {
				t.Errorf("failed steps = %v", note.FailedSteps)
			}
			if stepSaved(t, workDir, tt.failedFile) || !stepSaved(t, workDir, tt.savedFile) || !stepSaved(t, workDir, notesFile) {
				t.Error("only the failed step should be missing")
			}

			fake.setFailing()
			retried, err := MakeNote(context.Background(), client, testLink, llm.NEET, workDir)
			if err != nil {
				t.Fatal(err)
			}
			if len(retried.FailedSteps) != 0 || len(retried.Flashcards) != 10 || len(retried.Quiz) != 5 {
				t.Errorf("retried = %+v", retried)
			}
			if fake.total() != 4 || fake.step(tt.failing) != 2 {
				t.Errorf("calls = %d, %s calls = %d, only the failed step should run again", fake.total(), tt.failing, fake.step(tt.failing))
			}
		})
	}
}

func TestNotesFailureStillSavesTheOtherSteps(t *testing.T) {
	client, fake := fakeGemini(t)
	fake.setFailing("notes")
	workDir := workDirWithTranscript(t)

	if _, err := MakeNote(context.Background(), client, testLink, llm.NEET, workDir); err == nil {
		t.Fatal("no note means nothing to show, so this must be an error")
	}
	if !stepSaved(t, workDir, flashcardsFile) || !stepSaved(t, workDir, quizFile) {
		t.Error("flashcards and quiz don't depend on the notes")
	}

	fake.setFailing()
	if _, err := MakeNote(context.Background(), client, testLink, llm.NEET, workDir); err != nil {
		t.Fatal(err)
	}
	if fake.step("notes") != 2 || fake.step("flashcards") != 1 || fake.step("quiz") != 1 {
		t.Errorf("calls = %v", fake.calls)
	}
}

func TestArraysAreNeverNull(t *testing.T) {
	client, fake := fakeGemini(t)
	fake.setFailing("flashcards", "quiz")

	note, err := MakeNote(context.Background(), client, testLink, llm.NEET, workDirWithTranscript(t))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(note)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "null") {
		t.Errorf("note has a null: %s", data)
	}
}

func TestBadLinkStopsBeforeAnyWork(t *testing.T) {
	workDir := t.TempDir()
	_, err := MakeNote(context.Background(), nil, "https://example.com/video", llm.NEET, workDir)
	if !errors.Is(err, captions.ErrBadLink) {
		t.Errorf("err = %v", err)
	}
	if entries, _ := os.ReadDir(workDir); len(entries) != 0 {
		t.Errorf("nothing should be created, found %v", entries)
	}
}
