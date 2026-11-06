package pipeline

import (
	"errors"

	"warden/internal/pipeline/captions"
	"warden/internal/pipeline/llm"
)

// PlainError is what a student reads when a step fails. Raw errors can hold
// Gemini's JSON, so they stay in the logs.
func PlainError(step string, err error) string {
	switch {
	case errors.Is(err, llm.ErrDailyQuota):
		return "We've used up today's AI limit. Please try again tomorrow."
	case errors.Is(err, llm.ErrBusy):
		return "The AI service is busy right now. Please try again in a few minutes."
	case errors.Is(err, llm.ErrChecks):
		return "We couldn't make this lecture's " + step + ". Please try again."
	case errors.Is(err, captions.ErrBadLink):
		return "That isn't a YouTube video link."
	case errors.Is(err, captions.ErrTooLong):
		return "This video is over the 4 hour limit."
	case errors.Is(err, captions.ErrNoLength):
		return "This video has no fixed length. It may be a live stream."
	case errors.Is(err, captions.ErrNoCaptions):
		return "This video has no English or Hindi captions."
	case step == captionsStep.Name, errors.Is(err, errNoTranscript):
		return "We couldn't read this video from YouTube. Please try again."
	}
	return "Something went wrong making the " + step + ". Please try again."
}
