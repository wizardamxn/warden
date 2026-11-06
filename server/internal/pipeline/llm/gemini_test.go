package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"warden/internal/notes"
	"warden/internal/pipeline/captions"
)

// fakeGemini answers each call with the next reply, and keeps the requests.
func fakeGemini(t *testing.T, replies ...string) (*Client, *[]generateRequest) {
	t.Helper()
	var requests []generateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("api key header = %q", r.Header.Get("x-goog-api-key"))
		}
		if strings.Contains(r.URL.String(), "test-key") {
			t.Error("api key is in the URL")
		}
		if want := "/models/" + model + ":generateContent"; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		var request generateRequest
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error(err)
		}
		requests = append(requests, request)
		if len(requests) > len(replies) {
			t.Error("more calls than expected")
			http.Error(w, "no reply left", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, replies[len(requests)-1])
	}))
	t.Cleanup(server.Close)
	return &Client{Key: "test-key", BaseURL: server.URL, HTTP: server.Client()}, &requests
}

// reply wraps text the way Gemini does.
func reply(text string) string {
	wrapped, _ := json.Marshal(map[string]any{
		"candidates": []any{map[string]any{
			"content":      map[string]any{"parts": []any{map[string]any{"text": text}}},
			"finishReason": "STOP",
		}},
		"usageMetadata": map[string]any{"promptTokenCount": 1000, "candidatesTokenCount": 100, "thoughtsTokenCount": 10},
	})
	return string(wrapped)
}

func TestGenerateReadsTheAnswerAndSkipsThoughts(t *testing.T) {
	client, requests := fakeGemini(t, `{
		"candidates": [{
			"content": {"parts": [{"text": "thinking", "thought": true}, {"text": "{\"a\":"}, {"text": "1}"}]},
			"finishReason": "STOP"
		}],
		"usageMetadata": {"promptTokenCount": 2000000, "candidatesTokenCount": 800000, "thoughtsTokenCount": 200000}
	}`)
	text, usage, err := client.generate(context.Background(), "be brief", []turn{userTurn("hi")}, `{"type":"object"}`)
	if err != nil {
		t.Fatal(err)
	}
	if text != `{"a":1}` {
		t.Errorf("text = %q", text)
	}
	if usage.InputTokens != 2000000 || usage.OutputTokens != 1000000 {
		t.Errorf("usage = %+v", usage)
	}
	if got, want := usage.CostUSD(), 2.0; got != want {
		t.Errorf("cost = %v, want %v", got, want)
	}

	sent := (*requests)[0]
	if sent.GenerationConfig.ResponseMimeType != "application/json" || string(sent.GenerationConfig.ResponseJSONSchema) != `{"type":"object"}` {
		t.Errorf("config = %+v", sent.GenerationConfig)
	}
	if sent.SystemInstruction.Parts[0].Text != "be brief" {
		t.Errorf("instructions = %+v", sent.SystemInstruction)
	}
}

func TestGenerateErrors(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		want  string
	}{
		{"cut off", `{"candidates":[{"finishReason":"MAX_TOKENS"}]}`, "stopped early: MAX_TOKENS"},
		{"blocked", `{"promptFeedback":{"blockReason":"SAFETY"}}`, "no answer"},
		{"not json", `oops`, "read gemini response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := fakeGemini(t, tt.reply)
			_, _, err := client.generate(context.Background(), "", nil, `{}`)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestGenerateReportsHTTPErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer server.Close()
	client := &Client{Key: "secret-key", BaseURL: server.URL, HTTP: server.Client()}

	_, _, err := client.generate(context.Background(), "", nil, `{}`)
	if err == nil || !strings.Contains(err.Error(), "404") || !strings.Contains(err.Error(), "model not found") {
		t.Errorf("err = %v", err)
	}
	if strings.Contains(err.Error(), "secret-key") {
		t.Error("the error leaks the api key")
	}
}

func TestGenerateRetriesWhenBusy(t *testing.T) {
	tests := []struct {
		name      string
		busyFor   int
		wantCalls int
		wantErr   bool
	}{
		{name: "busy then fine", busyFor: 2, wantCalls: 3},
		{name: "busy until it gives up", busyFor: 99, wantCalls: maxAttempts, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls <= tt.busyFor {
					http.Error(w, "high demand", http.StatusServiceUnavailable)
					return
				}
				fmt.Fprint(w, reply(`{}`))
			}))
			defer server.Close()
			client := &Client{Key: "test-key", BaseURL: server.URL, HTTP: server.Client()}

			_, _, err := client.generate(context.Background(), "", nil, `{}`)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v", err)
			}
			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
		})
	}
}

// quotaBody is Google's 429 body, cut down to the parts that matter.
func quotaBody(quotaID, retryDelay string) string {
	return fmt.Sprintf(`{"error":{"code":429,"message":"You exceeded your current quota.","status":"RESOURCE_EXHAUSTED","details":[
		{"@type":"type.googleapis.com/google.rpc.QuotaFailure","violations":[{"quotaId":%q}]},
		{"@type":"type.googleapis.com/google.rpc.RetryInfo","retryDelay":%q}]}}`, quotaID, retryDelay)
}

func TestGenerateOnRateLimits(t *testing.T) {
	const perDay = "GenerateRequestsPerDayPerProjectPerModel-FreeTier"
	const perMinute = "GenerateRequestsPerMinutePerProjectPerModel-FreeTier"
	tests := []struct {
		name      string
		status    int
		body      string
		wantCalls int
		wantErr   error
		wantWait  time.Duration
	}{
		{name: "daily quota stops at once", status: 429, body: quotaBody(perDay, "40s"), wantCalls: 1, wantErr: ErrDailyQuota},
		{name: "per-minute limit waits for the hint", status: 429, body: quotaBody(perMinute, "0.06s"), wantCalls: 3, wantWait: 120 * time.Millisecond},
		{name: "429 that names no limit is retried", status: 429, body: `{"error":{"code":429}}`, wantCalls: 3},
		{name: "429 that is not json is retried", status: 429, body: `slow down`, wantCalls: 3},
		{name: "503 is retried", status: 503, body: `{"error":{"code":503,"message":"high demand"}}`, wantCalls: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls < 3 {
					http.Error(w, tt.body, tt.status)
					return
				}
				fmt.Fprint(w, reply(`{}`))
			}))
			defer server.Close()
			client := &Client{Key: "secret-key", BaseURL: server.URL, HTTP: server.Client()}

			started := time.Now()
			_, _, err := client.generate(context.Background(), "", nil, `{}`)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
			if time.Since(started) < tt.wantWait {
				t.Errorf("waited %s, want at least %s", time.Since(started), tt.wantWait)
			}
			if err != nil && strings.Contains(err.Error(), "secret-key") {
				t.Error("the error leaks the api key")
			}
		})
	}
}

func TestLongRetryHintIsCapped(t *testing.T) {
	if got := min(retryHint([]byte(quotaBody("x", "86400s"))), maxRetryWait); got != maxRetryWait {
		t.Errorf("wait = %s", got)
	}
}

func TestBusyGeminiEndsWithErrBusy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "high demand", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := &Client{Key: "test-key", BaseURL: server.URL, HTTP: server.Client()}

	if _, _, err := client.generate(context.Background(), "", nil, `{}`); !errors.Is(err, ErrBusy) {
		t.Errorf("err = %v", err)
	}
}

func TestGenerateDoesNotRetryOtherErrors(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()
	client := &Client{Key: "test-key", BaseURL: server.URL, HTTP: server.Client()}

	if _, _, err := client.generate(context.Background(), "", nil, `{}`); err == nil {
		t.Error("expected an error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestGenerateNeedsAKey(t *testing.T) {
	_, _, err := NewClient("").generate(context.Background(), "", nil, `{}`)
	if err == nil || !strings.Contains(err.Error(), "server/.env") {
		t.Errorf("err = %v", err)
	}
}

var testVideo = captions.Video{ID: "VjnHhmST8GY", Title: "Immunity", Channel: "Test", DurationSeconds: 100, Source: "auto-captions"}

const goodAnswer = `{
	"video_id": "madeUpByAI1",
	"title": "Made up by the AI",
	"ncert_chapter": "Human Health and Disease",
	"tldr": "What immunity is.",
	"key_points": ["Two types."],
	"sections": [{"start_seconds": 0, "title": "Immunity", "bullets": ["Two types."], "exam_mark": "", "exam_quote": ""}],
	"mnemonics": [{"topic": "Antibody classes", "text": "GAMED"}],
	"glossary": []
}`

func TestMakeNote(t *testing.T) {
	client, requests := fakeGemini(t, reply(goodAnswer))
	note, usage, err := MakeNote(context.Background(), client, testVideo, testLines)
	if err != nil {
		t.Fatal(err)
	}
	if note.VideoID != "VjnHhmST8GY" || note.Title != "Immunity" || note.Channel != "Test" || note.DurationSeconds != 100 || note.Source != "auto-captions" {
		t.Errorf("our own fields weren't filled in: %+v", note)
	}
	if note.NCERTChapter != "Human Health and Disease" || len(note.Mnemonics) != 1 {
		t.Errorf("the new fields weren't read: %+v", note)
	}
	if usage.InputTokens != 1000 || usage.OutputTokens != 110 {
		t.Errorf("usage = %+v", usage)
	}
	prompt := (*requests)[0].Contents[0].Parts[0].Text
	if !strings.Contains(prompt, "[40] ये नीट में क्वेश्चन आ चुका है") {
		t.Errorf("the transcript isn't in the prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "Write about 3 sections") {
		t.Errorf("the section count isn't in the prompt: %q", prompt)
	}
}

func TestMakeNoteSendsProblemsBackOnce(t *testing.T) {
	bad := `{"tldr": "", "key_points": ["x"], "sections": [{"start_seconds": 0, "title": "t", "bullets": ["b"]}], "glossary": []}`
	client, requests := fakeGemini(t, reply(bad), reply(goodAnswer))
	note, usage, err := MakeNote(context.Background(), client, testVideo, testLines)
	if err != nil {
		t.Fatal(err)
	}
	if note.TLDR != "What immunity is." {
		t.Errorf("tldr = %q", note.TLDR)
	}
	if usage.InputTokens != 2000 {
		t.Errorf("both calls should count: %+v", usage)
	}
	second := (*requests)[1].Contents
	if len(second) != 3 || second[1].Role != "model" || !strings.Contains(second[2].Parts[0].Text, "tldr is empty") {
		t.Errorf("second call = %+v", second)
	}
}

func TestMakeNoteDropsAnExamMarkItCannotVerify(t *testing.T) {
	invented := `{
		"tldr": "What immunity is.",
		"key_points": ["Two types."],
		"sections": [{"start_seconds": 0, "title": "Immunity", "bullets": ["Two types."],
			"exam_mark": "Asked 2024", "exam_quote": "made up words said in 2024"}],
		"glossary": []
	}`
	client, _ := fakeGemini(t, reply(invented), reply(invented))
	note, _, err := MakeNote(context.Background(), client, testVideo, testLines)
	if err != nil {
		t.Fatal(err)
	}
	if note.TLDR == "" || note.Sections[0].ExamMark != "" || note.Sections[0].ExamQuote != "" {
		t.Errorf("note = %+v", note)
	}
}

func TestMakeNoteKeepsAProvenExamMark(t *testing.T) {
	proven := `{
		"tldr": "What immunity is.",
		"key_points": ["Two types."],
		"sections": [{"start_seconds": 0, "title": "Immunity", "bullets": ["Two types."],
			"exam_mark": "Asked 2024", "exam_quote": "ये नीट में क्वेश्चन आ चुका है 2024 में"}],
		"glossary": []
	}`
	client, _ := fakeGemini(t, reply(proven))
	note, _, err := MakeNote(context.Background(), client, testVideo, testLines)
	if err != nil {
		t.Fatal(err)
	}
	if note.Sections[0].ExamMark != "Asked 2024" {
		t.Errorf("note = %+v", note)
	}
}

func TestMakeNoteOnlyRescuesAWholeAnswer(t *testing.T) {
	// Go reads the fields before the bad one, so this looks like a good note.
	cutShort := `{"tldr": "What immunity is.", "key_points": ["Two types."],
		"sections": [{"start_seconds": 0, "title": "Immunity", "bullets": ["Two types."]}], "glossary": "oops"}`
	client, _ := fakeGemini(t, reply(cutShort), reply(cutShort))
	if _, _, err := MakeNote(context.Background(), client, testVideo, testLines); err == nil {
		t.Error("expected an error")
	}
}

func TestMakeNoteGivesUpAfterTwoBadAnswers(t *testing.T) {
	client, requests := fakeGemini(t, reply(`not json`), reply(`{"tldr": ""}`))
	_, usage, err := MakeNote(context.Background(), client, testVideo, testLines)
	if err == nil || !strings.Contains(err.Error(), "tldr is empty") {
		t.Errorf("err = %v", err)
	}
	if len(*requests) != 2 {
		t.Errorf("calls = %d, want 2", len(*requests))
	}
	if usage.InputTokens != 2000 {
		t.Errorf("failed calls still cost money, usage = %+v", usage)
	}
}

// answerText writes a list the way Gemini would.
func answerText(t *testing.T, list any) string {
	t.Helper()
	data, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMakeFlashcards(t *testing.T) {
	client, requests := fakeGemini(t, reply(answerText(t, goodCards())))
	cards, usage, err := MakeFlashcards(context.Background(), client, testVideo, testLines)
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != minFlashcards || usage.InputTokens != 1000 {
		t.Errorf("cards = %d, usage = %+v", len(cards), usage)
	}
	if prompt := (*requests)[0].Contents[0].Parts[0].Text; !strings.Contains(prompt, "Write 10 flashcards") || !strings.Contains(prompt, "[40] ये नीट") {
		t.Errorf("prompt = %q", prompt)
	}
}

func TestShuffleOptions(t *testing.T) {
	seen := map[int]bool{}
	for range 200 {
		quiz := []notes.Question{
			{Type: "direct", Options: []string{"right", "b", "c", "d"}, AnswerIndex: 0},
			{Type: "assertion-reason", Options: []string{"1", "2", "3", "4"}, AnswerIndex: 2},
			{Type: "statement", Options: []string{"1", "2", "3", "4"}, AnswerIndex: 1},
		}
		shuffleOptions(quiz)
		if got := quiz[0].Options[quiz[0].AnswerIndex]; got != "right" {
			t.Fatalf("answer_index now points at %q", got)
		}
		seen[quiz[0].AnswerIndex] = true
		for _, q := range quiz[1:] {
			if !slices.Equal(q.Options, []string{"1", "2", "3", "4"}) {
				t.Fatalf("%s options moved: %v", q.Type, q.Options)
			}
		}
	}
	if len(seen) != optionCount {
		t.Errorf("the answer only landed in positions %v", seen)
	}
}

func TestMakeQuiz(t *testing.T) {
	client, requests := fakeGemini(t, reply(answerText(t, goodQuiz())))
	quiz, usage, err := MakeQuiz(context.Background(), client, testVideo, testLines)
	if err != nil {
		t.Fatal(err)
	}
	if len(quiz) != minQuestions || usage.InputTokens != 1000 {
		t.Errorf("quiz = %d, usage = %+v", len(quiz), usage)
	}
	if prompt := (*requests)[0].Contents[0].Parts[0].Text; !strings.Contains(prompt, "Write 5 questions") || !strings.Contains(prompt, "[40] ये नीट") {
		t.Errorf("prompt = %q", prompt)
	}
}

func TestStudyStepsSendProblemsBackOnce(t *testing.T) {
	badCards, badQuiz := goodCards(), goodQuiz()
	badCards[0].StartSeconds = 70 // too far from any line to snap
	badQuiz[0].AnswerIndex = 9

	tests := []struct {
		name    string
		run     func(client *Client) error
		bad     string
		good    string
		problem string
	}{
		{
			name: "flashcards",
			run: func(client *Client) error {
				_, _, err := MakeFlashcards(context.Background(), client, testVideo, testLines)
				return err
			},
			bad:     answerText(t, badCards),
			good:    answerText(t, goodCards()),
			problem: "flashcards[0].start_seconds 70",
		},
		{
			name: "quiz",
			run: func(client *Client) error {
				_, _, err := MakeQuiz(context.Background(), client, testVideo, testLines)
				return err
			},
			bad:     answerText(t, badQuiz),
			good:    answerText(t, goodQuiz()),
			problem: "quiz[0].answer_index 9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name+" fixed on the second try", func(t *testing.T) {
			client, requests := fakeGemini(t, reply(tt.bad), reply(tt.good))
			if err := tt.run(client); err != nil {
				t.Fatal(err)
			}
			if fix := (*requests)[1].Contents[2].Parts[0].Text; !strings.Contains(fix, tt.problem) {
				t.Errorf("fix-up = %q, want it to contain %q", fix, tt.problem)
			}
		})
		t.Run(tt.name+" stops after two bad answers", func(t *testing.T) {
			client, requests := fakeGemini(t, reply(tt.bad), reply(tt.bad))
			err := tt.run(client)
			if err == nil || !strings.Contains(err.Error(), tt.problem) {
				t.Errorf("err = %v", err)
			}
			if len(*requests) != 2 {
				t.Errorf("calls = %d, want 2", len(*requests))
			}
		})
	}
}
