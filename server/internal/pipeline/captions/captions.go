// Package captions gets a YouTube video's details and captions with yt-dlp.
package captions

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const maxDurationSeconds = 4 * 60 * 60

var (
	ErrBadLink    = errors.New("not a YouTube video link")
	ErrTooLong    = errors.New("video is over the 4 hour limit")
	ErrNoLength   = errors.New("video has no fixed length, it may be live")
	ErrNoCaptions = errors.New("video has no English or Hindi captions")
)

var videoIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// Video is what we know about a lecture before making its note.
type Video struct {
	ID              string
	Title           string
	Channel         string
	DurationSeconds int
	Source          string // "captions" or "auto-captions"
}

// Line is one caption line and the second of the video it starts at.
type Line struct {
	StartSeconds int
	Text         string
}

// videoInfo is the part of yt-dlp's JSON we use.
type videoInfo struct {
	Title        string                     `json:"title"`
	Channel      string                     `json:"channel"`
	Duration     float64                    `json:"duration"`
	Language     string                     `json:"language"`
	Subtitles    map[string]json.RawMessage `json:"subtitles"`
	AutoCaptions map[string]json.RawMessage `json:"automatic_captions"`
}

// VideoID pulls the video ID out of a link and rejects anything else.
// Only the ID reaches yt-dlp, so a pasted link can't smuggle in anything extra.
func VideoID(link string) (string, error) {
	link = strings.TrimSpace(link)
	if !strings.Contains(link, "://") {
		link = "https://" + link
	}
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", ErrBadLink
	}

	var id string
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch strings.ToLower(u.Hostname()) {
	case "youtu.be":
		id = parts[0]
	case "youtube.com", "www.youtube.com", "m.youtube.com":
		if parts[0] == "watch" {
			id = u.Query().Get("v")
		} else if len(parts) == 2 && slices.Contains([]string{"shorts", "live", "embed"}, parts[0]) {
			id = parts[1]
		}
	}
	if !videoIDPattern.MatchString(id) {
		return "", ErrBadLink
	}
	return id, nil
}

// Fetch gets a video's details and its raw caption lines.
// It works in dir, which the caller cleans up.
func Fetch(ctx context.Context, link, dir string) (Video, []Line, error) {
	id, err := VideoID(link)
	if err != nil {
		return Video{}, nil, err
	}
	watchURL := "https://www.youtube.com/watch?v=" + id

	out, err := ytDlp(ctx, "--skip-download", "--dump-json", watchURL)
	if err != nil {
		return Video{}, nil, err
	}
	var info videoInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return Video{}, nil, fmt.Errorf("read yt-dlp output: %w", err)
	}

	if info.Duration <= 0 {
		return Video{}, nil, ErrNoLength
	}
	if info.Duration > maxDurationSeconds {
		return Video{}, nil, ErrTooLong
	}
	lang, source, ok := chooseCaptions(info)
	if !ok {
		return Video{}, nil, ErrNoCaptions
	}

	_, err = ytDlp(ctx, "--skip-download", "--write-subs", "--write-auto-subs",
		"--sub-langs", lang, "--sub-format", "json3",
		"--paths", dir, "--output", "%(id)s.%(ext)s", watchURL)
	if err != nil {
		return Video{}, nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, id+"."+lang+".json3"))
	if err != nil {
		return Video{}, nil, fmt.Errorf("read captions: %w", err)
	}
	lines, err := parseJSON3(data)
	if err != nil {
		return Video{}, nil, err
	}

	video := Video{
		ID:              id,
		Title:           info.Title,
		Channel:         info.Channel,
		DurationSeconds: int(info.Duration),
		Source:          source,
	}
	return video, lines, nil
}

// chooseCaptions picks which captions to download: manual ones first, then
// YouTube's auto captions in the language the lecture was spoken in.
func chooseCaptions(info videoInfo) (lang, source string, ok bool) {
	for _, base := range []string{"en", "hi"} {
		for _, key := range slices.Sorted(maps.Keys(info.Subtitles)) {
			if key == base || strings.HasPrefix(key, base+"-") {
				return key, "captions", true
			}
		}
	}

	// Auto captions list about 150 machine translations. "-orig" marks the spoken one.
	candidates := []string{"hi-orig", "en-orig"}
	if base, _, _ := strings.Cut(info.Language, "-"); base == "en" || base == "hi" {
		candidates = append(candidates, info.Language, base)
	}
	for _, key := range candidates {
		if _, found := info.AutoCaptions[key]; found {
			return key, "auto-captions", true
		}
	}
	return "", "", false
}

// parseJSON3 reads YouTube's json3 format. Each event with text becomes one
// line, kept exactly as it came. Clean tidies them up.
func parseJSON3(data []byte) ([]Line, error) {
	var file struct {
		Events []struct {
			StartMs  int `json:"tStartMs"`
			Segments []struct {
				Text string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("read captions: %w", err)
	}

	var lines []Line
	for _, event := range file.Events {
		if len(event.Segments) == 0 {
			continue
		}
		var text strings.Builder
		for _, segment := range event.Segments {
			text.WriteString(segment.Text)
		}
		lines = append(lines, Line{StartSeconds: event.StartMs / 1000, Text: text.String()})
	}
	return lines, nil
}

// CookiesFile is a Netscape-format cookies.txt from a logged-in browser. YouTube
// asks servers to "sign in to confirm you're not a bot" without one. Empty means none.
var CookiesFile string

// ytDlp runs yt-dlp and returns what it printed.
func ytDlp(ctx context.Context, args ...string) ([]byte, error) {
	base := []string{"--no-warnings", "--no-playlist"}
	if CookiesFile != "" {
		base = append(base, "--cookies", CookiesFile)
	}
	cmd := exec.CommandContext(ctx, "yt-dlp", append(base, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}
