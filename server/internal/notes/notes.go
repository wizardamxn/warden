// Package notes holds the note's shape, so every part of the server uses the same one.
package notes

// The templates a student can pick. A template sets how notes, flashcards and
// the quiz are made, and how the website scores the quiz.
const (
	NEET    = "neet"
	General = "general"
)

// Templates lists every template.
var Templates = []string{NEET, General}

// Note is one finished note for one lecture.
type Note struct {
	VideoID         string         `json:"video_id"`
	Template        string         `json:"template"` // filled in by Go from the job, never by the AI
	Title           string         `json:"title"`
	Channel         string         `json:"channel"`
	DurationSeconds int            `json:"duration_seconds"`
	Source          string         `json:"source"`        // "captions" or "auto-captions"
	NCERTChapter    string         `json:"ncert_chapter"` // chapter name only, empty if unclear
	TLDR            string         `json:"tldr"`
	KeyPoints       []string       `json:"key_points"`
	Sections        []Section      `json:"sections"`
	Mnemonics       []Mnemonic     `json:"mnemonics"`
	Glossary        []GlossaryTerm `json:"glossary"`
	Flashcards      []Flashcard    `json:"flashcards"`
	Quiz            []Question     `json:"quiz"`
	FailedSteps     []string       `json:"failed_steps"` // steps with nothing to show, a re-run tries them again
}

// Section is one topic of the lecture, starting at a moment in the video.
type Section struct {
	StartSeconds int      `json:"start_seconds"`
	Title        string   `json:"title"`
	Bullets      []string `json:"bullets"`
	ExamMark     string   `json:"exam_mark"`  // what the teacher said about NEET here, or empty
	ExamQuote    string   `json:"exam_quote"` // the teacher's exact words, so the mark can be checked
}

// Mnemonic is one memory trick for one topic.
type Mnemonic struct {
	Topic string `json:"topic"`
	Text  string `json:"text"`
}

// GlossaryTerm is one term and what it means.
type GlossaryTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

// Flashcard tests one fact, and links back to where the lecture teaches it.
type Flashcard struct {
	Question     string `json:"question"`
	Answer       string `json:"answer"`
	StartSeconds int    `json:"start_seconds"`
}

// Question is one quiz question.
type Question struct {
	Type         string   `json:"type"` // "direct", "statement", and for NEET also "assertion-reason" or "match"
	Question     string   `json:"question"`
	Options      []string `json:"options"`      // always 4
	AnswerIndex  int      `json:"answer_index"` // 0 to 3
	Explanation  string   `json:"explanation"`
	StartSeconds int      `json:"start_seconds"`
}
