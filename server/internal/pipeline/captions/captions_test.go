package captions

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
)

func TestVideoID(t *testing.T) {
	const id = "VjnHhmST8GY"
	good := []string{
		"https://www.youtube.com/watch?v=" + id,
		"https://www.youtube.com/watch?v=" + id + "&t=120s&list=PLabc",
		"https://youtube.com/watch?v=" + id,
		"https://m.youtube.com/watch?v=" + id,
		"http://www.youtube.com/watch?v=" + id,
		"www.youtube.com/watch?v=" + id,
		"  https://www.youtube.com/watch?v=" + id + "  ",
		"https://youtu.be/" + id,
		"https://youtu.be/" + id + "?si=abc",
		"https://www.youtube.com/shorts/" + id,
		"https://www.youtube.com/live/" + id,
		"https://www.youtube.com/embed/" + id,
	}
	for _, link := range good {
		got, err := VideoID(link)
		if err != nil || got != id {
			t.Errorf("VideoID(%q) = %q, %v, want %q", link, got, err, id)
		}
	}

	bad := []string{
		"",
		"not a link",
		"https://example.com/watch?v=" + id,
		"https://youtube.com.evil.com/watch?v=" + id,
		"https://www.youtube.com/",
		"https://www.youtube.com/watch",
		"https://www.youtube.com/watch?v=short",
		"https://www.youtube.com/watch?v=--exec=calc",
		"https://www.youtube.com/playlist?list=PLabc",
		"https://www.youtube.com/@PhysicsWallah",
		"ftp://youtube.com/watch?v=" + id,
	}
	for _, link := range bad {
		if got, err := VideoID(link); !errors.Is(err, ErrBadLink) {
			t.Errorf("VideoID(%q) = %q, %v, want ErrBadLink", link, got, err)
		}
	}
}

func keys(names ...string) map[string]json.RawMessage {
	m := map[string]json.RawMessage{}
	for _, name := range names {
		m[name] = nil
	}
	return m
}

func TestChooseCaptions(t *testing.T) {
	tests := []struct {
		name       string
		info       videoInfo
		wantLang   string
		wantSource string
		wantOK     bool
	}{
		{
			name:       "manual English beats everything",
			info:       videoInfo{Subtitles: keys("hi", "en-IN"), AutoCaptions: keys("hi-orig")},
			wantLang:   "en-IN",
			wantSource: "captions",
			wantOK:     true,
		},
		{
			name:       "manual Hindi",
			info:       videoInfo{Subtitles: keys("hi"), AutoCaptions: keys("en-orig")},
			wantLang:   "hi",
			wantSource: "captions",
			wantOK:     true,
		},
		{
			name:       "spoken Hindi, not its English translation",
			info:       videoInfo{Language: "hi", AutoCaptions: keys("en", "hi", "hi-orig", "ta")},
			wantLang:   "hi-orig",
			wantSource: "auto-captions",
			wantOK:     true,
		},
		{
			name:       "spoken English",
			info:       videoInfo{Language: "en", AutoCaptions: keys("en", "en-orig", "hi")},
			wantLang:   "en-orig",
			wantSource: "auto-captions",
			wantOK:     true,
		},
		{
			name:       "no -orig, uses the video's language",
			info:       videoInfo{Language: "en-US", AutoCaptions: keys("en-US", "hi")},
			wantLang:   "en-US",
			wantSource: "auto-captions",
			wantOK:     true,
		},
		{
			name:   "another language only has translations for us",
			info:   videoInfo{Language: "ta", AutoCaptions: keys("en", "hi", "ta-orig")},
			wantOK: false,
		},
		{name: "no captions at all", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lang, source, ok := chooseCaptions(tt.info)
			if lang != tt.wantLang || source != tt.wantSource || ok != tt.wantOK {
				t.Errorf("got %q, %q, %v, want %q, %q, %v", lang, source, ok, tt.wantLang, tt.wantSource, tt.wantOK)
			}
		})
	}
}

func TestParseJSON3(t *testing.T) {
	data := []byte(`{"events":[
		{"tStartMs":0,"dDurationMs":4234920,"id":1},
		{"tStartMs":80,"segs":[{"utf8":"So"},{"utf8":" first","tOffsetMs":119}]},
		{"tStartMs":1910,"aAppend":1,"segs":[{"utf8":"\n"}]},
		{"tStartMs":61920,"segs":[{"utf8":"antigen"}]}
	]}`)
	want := []Line{
		{StartSeconds: 0, Text: "So first"},
		{StartSeconds: 1, Text: "\n"},
		{StartSeconds: 61, Text: "antigen"},
	}
	got, err := parseJSON3(data)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseJSON3RejectsBadData(t *testing.T) {
	if _, err := parseJSON3([]byte("not json")); err == nil {
		t.Error("expected an error")
	}
}
