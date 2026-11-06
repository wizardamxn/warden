package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"warden/internal/api"
	"warden/internal/core"
	"warden/internal/jobs"
)

func call(t *testing.T, server *httptest.Server, method, path, body string) (int, jobs.View, map[string]any) {
	t.Helper()
	request, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	var raw json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}
	var view jobs.View
	var fields map[string]any
	if err := json.Unmarshal(raw, &view); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, view, fields
}

func TestAPIContract(t *testing.T) {
	q := testQueue(t)
	client, fake := fakeGemini(t)
	server := httptest.NewServer(api.New(q, core.LocalWebOrigin))
	defer server.Close()

	const body = `{"url":"https://www.youtube.com/watch?v=` + testID + `"}`
	if code, _, fields := call(t, server, http.MethodGet, "/v1/notes/"+testID, ""); code != 404 || fields["error"] == "" {
		t.Errorf("unknown video: %d %v", code, fields)
	}
	if code, _, fields := call(t, server, http.MethodPost, "/v1/notes/"+testID+"/retry", ""); code != 404 || fields["error"] == "" {
		t.Errorf("unknown retry: %d %v", code, fields)
	}

	// First request: queued, nothing ready.
	code, view, fields := call(t, server, http.MethodPost, "/v1/notes", body)
	if code != 202 || view.VideoID != testID || view.Status != "queued" || view.Note != nil {
		t.Fatalf("first post: %d %+v", code, view)
	}
	if note, present := fields["note"]; !present || note != nil {
		t.Errorf("note must be there and null: %v", fields)
	}
	wantSteps := []string{"captions", "notes", "flashcards", "quiz"}
	if len(view.Steps) != len(wantSteps) {
		t.Fatalf("steps = %+v", view.Steps)
	}
	for i, name := range wantSteps {
		if view.Steps[i].Name != name || view.Steps[i].Status != "pending" {
			t.Errorf("step %d = %+v", i, view.Steps[i])
		}
	}

	// The worker's part, with a failure.
	seedVideo(t, q, testID)
	fake.fail(500, `{"error":{"message":"raw gemini text"}}`, "quiz")
	runNext(t, q, client)

	code, view, _ = call(t, server, http.MethodGet, "/v1/notes/"+testID, "")
	if code != 200 || view.Status != "partial" || view.Note == nil || view.Note.FailedSteps[0] != "quiz" {
		t.Fatalf("get: %d %+v", code, view)
	}
	if err := view.Steps[3].Error; err == "" || strings.Contains(err, "raw gemini") {
		t.Errorf("quiz error = %q", err)
	}
	if view.Error != "" {
		t.Errorf("a partial note has no job error, got %q", view.Error)
	}

	// Retry queues only the failed step.
	fake.fail(0, "")
	code, view, _ = call(t, server, http.MethodPost, "/v1/notes/"+testID+"/retry", "")
	if code != 202 || view.Status != "queued" || view.Steps[3].Status != "pending" || view.Steps[2].Status != "done" {
		t.Fatalf("retry: %d %+v", code, view)
	}
	runNext(t, q, client)

	// A finished video answers 200 straight away, with no AI calls.
	code, view, fields = call(t, server, http.MethodPost, "/v1/notes", body)
	if code != 200 || view.Status != "done" || view.Note == nil || len(view.Note.Quiz) != 5 {
		t.Fatalf("second post: %d %+v", code, view)
	}
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "null") {
		t.Errorf("no field may be null once a note exists: %s", data)
	}
	wantNothingToClaim(t, q)
	fake.wantCalls(t, map[string]int{"notes": 1, "flashcards": 1, "quiz": 2})
}
