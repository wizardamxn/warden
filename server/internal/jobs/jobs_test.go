package jobs

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOverallStatus(t *testing.T) {
	tests := []struct {
		name     string
		statuses [4]string // captions, notes, flashcards, quiz
		held     bool
		want     string
	}{
		{name: "nothing started", statuses: [4]string{"pending", "pending", "pending", "pending"}, want: "queued"},
		{name: "a step is running", statuses: [4]string{"running", "pending", "pending", "pending"}, held: true, want: "running"},
		{name: "between two steps", statuses: [4]string{"done", "done", "pending", "pending"}, held: true, want: "running"},
		{name: "dead worker's step still counts until taken back", statuses: [4]string{"done", "running", "pending", "pending"}, want: "running"},
		{name: "retry is waiting", statuses: [4]string{"done", "done", "pending", "done"}, want: "queued"},
		{name: "everything done", statuses: [4]string{"done", "done", "done", "done"}, want: "done"},
		{name: "flashcards failed", statuses: [4]string{"done", "done", "failed", "done"}, want: "partial"},
		{name: "quiz failed", statuses: [4]string{"done", "done", "done", "failed"}, want: "partial"},
		{name: "notes failed, others saved", statuses: [4]string{"done", "failed", "done", "done"}, want: "failed"},
		{name: "captions failed", statuses: [4]string{"failed", "failed", "failed", "failed"}, want: "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps := []StepView{
				{Name: "captions", Status: tt.statuses[0]},
				{Name: "notes", Status: tt.statuses[1]},
				{Name: "flashcards", Status: tt.statuses[2]},
				{Name: "quiz", Status: tt.statuses[3]},
			}
			if got := overallStatus(steps, tt.held); got != tt.want {
				t.Errorf("status = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildViewShowsTheNoteAsSoonAsItExists(t *testing.T) {
	view, err := buildView("abc", true, []stepRow{
		{Name: "captions", Status: "done"},
		{Name: "notes", Status: "done", Result: []byte(`{"video_id":"abc","title":"Immunity"}`)},
		{Name: "flashcards", Status: "failed", Error: "Plain words."},
		{Name: "quiz", Status: "running"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Note == nil || view.Note.Title != "Immunity" {
		t.Fatalf("note = %+v", view.Note)
	}
	if got := view.Note.FailedSteps; len(got) != 1 || got[0] != "flashcards" {
		t.Errorf("failed steps = %v", got)
	}
	data, err := json.Marshal(view.Note)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"flashcards":[]`, `"quiz":[]`, `"mnemonics":[]`, `"glossary":[]`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("note is missing %s: %s", want, data)
		}
	}
}

func TestBuildViewWithoutANote(t *testing.T) {
	view, err := buildView("abc", false, []stepRow{
		{Name: "captions", Status: "done"},
		{Name: "notes", Status: "failed", Error: "Plain words."},
		{Name: "flashcards", Status: "done", Result: []byte(`[]`)},
		{Name: "quiz", Status: "done", Result: []byte(`[]`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Status != "failed" || view.Note != nil || view.Error != "Plain words." {
		t.Errorf("view = %+v", view)
	}
}
