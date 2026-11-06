package pipeline

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"warden/internal/pipeline/captions"
	"warden/internal/pipeline/llm"
)

func TestPlainErrorNeverShowsTheRawError(t *testing.T) {
	const raw = `gemini returned 429: {"error":{"code":429}}`
	tests := []struct {
		name string
		step string
		err  error
		want string
	}{
		{name: "daily quota", step: "notes", err: fmt.Errorf("%w: %s", llm.ErrDailyQuota, raw), want: "tomorrow"},
		{name: "busy", step: "quiz", err: fmt.Errorf("%w: %s", llm.ErrBusy, raw), want: "busy"},
		{name: "failed checks", step: "flashcards", err: fmt.Errorf("flashcards %w: %s", llm.ErrChecks, raw), want: "this lecture's flashcards"},
		{name: "no captions", step: "captions", err: captions.ErrNoCaptions, want: "no English or Hindi captions"},
		{name: "too long", step: "captions", err: captions.ErrTooLong, want: "4 hour"},
		{name: "yt-dlp broke", step: "captions", err: errors.New("yt-dlp: " + raw), want: "from YouTube"},
		{name: "anything else", step: "quiz", err: errors.New(raw), want: "making the quiz"},
		{name: "notes after no captions", step: "notes", err: fmt.Errorf("%w: %w", errNoTranscript, captions.ErrNoCaptions), want: "no English or Hindi captions"},
		{name: "notes after yt-dlp broke", step: "notes", err: fmt.Errorf("%w: %s", errNoTranscript, raw), want: "from YouTube"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlainError(tt.step, tt.err)
			if !strings.Contains(got, tt.want) {
				t.Errorf("got %q, want it to contain %q", got, tt.want)
			}
			if strings.Contains(got, "{") || strings.Contains(got, "gemini") {
				t.Errorf("raw error reached the student: %q", got)
			}
		})
	}
}
