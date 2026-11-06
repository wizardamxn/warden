package captions

import (
	"os"
	"slices"
	"testing"
)

func TestClean(t *testing.T) {
	tests := []struct {
		name string
		in   []Line
		want []Line
	}{
		{
			name: "drops lines with nothing in them",
			in:   []Line{{1, "\n"}, {2, "immunity"}, {3, "   "}, {4, ""}},
			want: []Line{{2, "immunity"}},
		},
		{
			name: "trims the ends and squeezes spaces and newlines in the middle",
			in:   []Line{{5, "  T  cells\nand   B cells \n"}},
			want: []Line{{5, "T cells and B cells"}},
		},
		{
			name: "drops a line that repeats the one before it, keeping the first time",
			in:   []Line{{10, "antigen"}, {12, "antigen"}, {14, "antibody"}},
			want: []Line{{10, "antigen"}, {14, "antibody"}},
		},
		{
			name: "spots a repeat even with an empty line between them",
			in:   []Line{{10, "antigen"}, {11, "\n"}, {12, " antigen "}},
			want: []Line{{10, "antigen"}},
		},
		{
			name: "keeps a repeat that isn't right after the same line",
			in:   []Line{{10, "theek hai"}, {20, "antigen"}, {30, "theek hai"}},
			want: []Line{{10, "theek hai"}, {20, "antigen"}, {30, "theek hai"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Clean(tt.in); !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCleanEmpty(t *testing.T) {
	if got := Clean(nil); len(got) != 0 {
		t.Errorf("got %v, want nothing", got)
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		in   []Line
		want string
	}{
		{name: "nothing", in: nil, want: ""},
		{name: "start of the video", in: []Line{{0, "hello"}}, want: "[0] hello\n"},
		{name: "past an hour, still plain seconds", in: []Line{{3940, "antibody"}}, want: "[3940] antibody\n"},
		{
			name: "one line each",
			in:   []Line{{9, "one"}, {65, "two"}},
			want: "[9] one\n[65] two\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A small file in the same shape YouTube really sends: every line of speech
// is followed by an event holding only a newline.
func TestSavedCaptionsBecomeATranscript(t *testing.T) {
	data, err := os.ReadFile("testdata/sample.json3")
	if err != nil {
		t.Fatal(err)
	}
	lines, err := parseJSON3(data)
	if err != nil {
		t.Fatal(err)
	}
	want := "[0] so today we start immunity\n" +
		"[4] antigen ko pehchanta hai\n" +
		"[70] B cells make antibodies\n"
	if got := Format(Clean(lines)); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
