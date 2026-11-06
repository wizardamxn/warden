package captions

import (
	"fmt"
	"strings"
)

// Clean tidies each line's spacing, then drops empty lines and any line that
// repeats the one before it. Half of YouTube's raw lines are only a newline.
func Clean(lines []Line) []Line {
	var cleaned []Line
	for _, line := range lines {
		text := strings.Join(strings.Fields(line.Text), " ")
		if text == "" {
			continue
		}
		if len(cleaned) > 0 && cleaned[len(cleaned)-1].Text == text {
			continue
		}
		cleaned = append(cleaned, Line{StartSeconds: line.StartSeconds, Text: text})
	}
	return cleaned
}

// Format writes lines as a transcript for the AI, one "[655] text" line each.
// Plain seconds on purpose: given "[10:55]", Gemini wrote 1055 instead of 655.
func Format(lines []Line) string {
	var transcript strings.Builder
	for _, line := range lines {
		fmt.Fprintf(&transcript, "[%d] %s\n", line.StartSeconds, line.Text)
	}
	return transcript.String()
}
