package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"warden/internal/notes"
	"warden/internal/pipeline/captions"
)

var testLines = []captions.Line{
	{StartSeconds: 0, Text: "आज हम इम्यूनिटी पढ़ेंगे"},
	{StartSeconds: 40, Text: "ये नीट में क्वेश्चन आ चुका है"},
	{StartSeconds: 44, Text: "2024 में और 2018 में भी आया था"},
}

func goodNote() notes.Note {
	return notes.Note{
		DurationSeconds: 100,
		TLDR:            "What immunity is.",
		KeyPoints:       []string{"T cells differentiate in the thymus."},
		Sections: []notes.Section{
			{StartSeconds: 0, Title: "Immunity", Bullets: []string{"Two types."}},
			{
				StartSeconds: 40,
				Title:        "Autoimmune disorders",
				Bullets:      []string{"Examples include lupus."},
				ExamMark:     "Asked 2018, 2024",
				ExamQuote:    "ये नीट में क्वेश्चन आ चुका है 2024 में और 2018 में भी आया था",
			},
		},
		Mnemonics: []notes.Mnemonic{{Topic: "Antibody classes", Text: "GAMED: IgG, IgA, IgM, IgE, IgD."}},
		Glossary:  []notes.GlossaryTerm{{Term: "Antigen", Definition: "Triggers an immune response."}},
	}
}

func goodCards() []notes.Flashcard {
	cards := make([]notes.Flashcard, minFlashcards)
	for i := range cards {
		cards[i] = notes.Flashcard{Question: "Where do T cells differentiate?", Answer: "Thymus", StartSeconds: 40}
	}
	return cards
}

func goodQuiz() []notes.Question {
	quiz := make([]notes.Question, minQuestions)
	for i := range quiz {
		quiz[i] = notes.Question{
			Type:         NEET.QuizTypes[i%len(NEET.QuizTypes)],
			Question:     "Statement I: T cells form in the bone marrow.\nStatement II: They differentiate in the thymus.",
			Options:      []string{"Thymus", "Bone marrow", "Spleen", "Liver"},
			AnswerIndex:  1,
			Explanation:  "T cells mature in the thymus, while B cells mature in the bone marrow.",
			StartSeconds: 40,
		}
	}
	return quiz
}

func TestSnapToLine(t *testing.T) {
	lines := []captions.Line{{StartSeconds: 150}, {StartSeconds: 156}, {StartSeconds: 159}, {StartSeconds: 200}}
	tests := []struct {
		name    string
		seconds int
		want    int
	}{
		{"already on a line", 156, 156},
		{"between two lines goes back to the one being spoken", 158, 156},
		{"at the limit", 164, 159},
		{"too far from any line stays put", 165, 165},
		{"before the first line stays put", 10, 10},
		{"past the last line by a little", 203, 200},
		{"far past the end stays put for the checks", 900, 900},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := snapToLine(tt.seconds, lines); got != tt.want {
				t.Errorf("snapToLine(%d) = %d, want %d", tt.seconds, got, tt.want)
			}
		})
	}
	if got := snapToLine(5, nil); got != 5 {
		t.Errorf("no lines: got %d, want 5", got)
	}
}

func TestStudyStepsSnapTimesWithoutAFixUp(t *testing.T) {
	cards := goodCards()
	cards[0].StartSeconds = 42 // between the lines at 40 and 44
	answer, err := json.Marshal(cards)
	if err != nil {
		t.Fatal(err)
	}
	client, requests := fakeGemini(t, reply(string(answer)))
	got, _, err := MakeFlashcards(context.Background(), client, NEET, captions.Video{DurationSeconds: 100}, testLines)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].StartSeconds != 40 {
		t.Errorf("start_seconds = %d, want 40", got[0].StartSeconds)
	}
	if len(*requests) != 1 {
		t.Errorf("calls = %d, want 1", len(*requests))
	}
}

func TestCheckNotePassesAGoodNote(t *testing.T) {
	if problems := CheckNote(goodNote(), testLines); len(problems) != 0 {
		t.Errorf("unexpected problems: %v", problems)
	}
}

func TestCheckNoteFindsProblems(t *testing.T) {
	tests := []struct {
		name   string
		change func(n *notes.Note)
		want   string
	}{
		{"missing tldr", func(n *notes.Note) { n.TLDR = " " }, "tldr is empty"},
		{"no key points", func(n *notes.Note) { n.KeyPoints = nil }, "key_points is empty"},
		{"no sections", func(n *notes.Note) { n.Sections = nil }, "sections is empty"},
		{"section without a title", func(n *notes.Note) { n.Sections[0].Title = "" }, "sections[0].title is empty"},
		{"section without bullets", func(n *notes.Note) { n.Sections[0].Bullets = nil }, "sections[0].bullets is empty"},
		{"time past the end", func(n *notes.Note) { n.Sections[1].StartSeconds = 101 }, "outside the lecture"},
		{"negative time", func(n *notes.Note) { n.Sections[0].StartSeconds = -1 }, "outside the lecture"},
		{"times out of order", func(n *notes.Note) { n.Sections[1].StartSeconds = 0 }, "must be after"},
		{"time on no transcript line", func(n *notes.Note) { n.Sections[1].StartSeconds = 55 }, "not the number on any transcript line"},
		{"Hindi in a bullet", func(n *notes.Note) { n.Sections[0].Bullets[0] = "शुक्राणु" }, "must be in English"},
		{"Hindi in the glossary", func(n *notes.Note) { n.Glossary[0].Term = "शुक्राणु" }, "must be in English"},
		{"Hindi in the chapter", func(n *notes.Note) { n.NCERTChapter = "कोशिका" }, "ncert_chapter must be in English"},
		{"page number in the chapter", func(n *notes.Note) { n.NCERTChapter = "Biomolecules, page 142" }, "chapter name only"},
		{"Hindi in a mnemonic", func(n *notes.Note) { n.Mnemonics[0].Text = "गेम्ड" }, "mnemonics[0].text must be in English"},
		{"mnemonic without a topic", func(n *notes.Note) { n.Mnemonics[0].Topic = "" }, "mnemonics[0].topic is empty"},
		{"Hindi in an exam mark", func(n *notes.Note) { n.Sections[1].ExamMark = "2024 में" }, "must be in English"},
		{"mark without a quote", func(n *notes.Note) { n.Sections[1].ExamQuote = "" }, "both exam_mark and exam_quote"},
		{"quote without a mark", func(n *notes.Note) { n.Sections[1].ExamMark = "" }, "both exam_mark and exam_quote"},
		{"invented quote", func(n *notes.Note) {
			n.Sections[1].ExamQuote = "ये बहुत इंपॉर्टेंट क्वेश्चन है"
		}, "not in the transcript"},
		{"quote too short", func(n *notes.Note) { n.Sections[1].ExamQuote = "2024 में" }, "too short"},
		{"year the teacher never said", func(n *notes.Note) { n.Sections[1].ExamMark = "Asked 2021" }, "names 2021"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := goodNote()
			tt.change(&note)
			problems := CheckNote(note, testLines)
			if !strings.Contains(strings.Join(problems, "\n"), tt.want) {
				t.Errorf("problems %v, want one containing %q", problems, tt.want)
			}
		})
	}
}

func TestCheckFlashcards(t *testing.T) {
	tests := []struct {
		name   string
		change func(c *[]notes.Flashcard)
		want   string
	}{
		{"good cards", func(c *[]notes.Flashcard) {}, ""},
		{"a few over the target", func(c *[]notes.Flashcard) { *c = append(*c, (*c)[0], (*c)[0]) }, ""},
		{"too few", func(c *[]notes.Flashcard) { *c = (*c)[:minFlashcards-1] }, "flashcards has 9 items, it needs 10"},
		{"too many", func(c *[]notes.Flashcard) { *c = append(*c, goodCards()...) }, "flashcards has 20 items"},
		{"no answer", func(c *[]notes.Flashcard) { (*c)[3].Answer = " " }, "flashcards[3].answer is empty"},
		{"Hindi in a question", func(c *[]notes.Flashcard) { (*c)[0].Question = "शुक्राणु क्या है" }, "must be in English"},
		{"time on no transcript line", func(c *[]notes.Flashcard) { (*c)[0].StartSeconds = 41 }, "not the number on any transcript line"},
		{"time past the end", func(c *[]notes.Flashcard) { (*c)[0].StartSeconds = 101 }, "outside the lecture"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cards := goodCards()
			tt.change(&cards)
			problems := CheckFlashcards(cards, 100, testLines)
			if tt.want == "" {
				if len(problems) != 0 {
					t.Errorf("unexpected problems: %v", problems)
				}
				return
			}
			if !strings.Contains(strings.Join(problems, "\n"), tt.want) {
				t.Errorf("problems %v, want one containing %q", problems, tt.want)
			}
		})
	}
}

func TestCheckQuiz(t *testing.T) {
	tests := []struct {
		name   string
		change func(q *[]notes.Question)
		want   string
	}{
		{"good quiz", func(q *[]notes.Question) {}, ""},
		{"too few", func(q *[]notes.Question) { *q = (*q)[:minQuestions-1] }, "quiz has 4 items, it needs 5"},
		{"unknown type", func(q *[]notes.Question) { (*q)[0].Type = "true-false" }, `quiz[0].type "true-false"`},
		{"a type missing from the mix", func(q *[]notes.Question) {
			(*q)[1].Type = "direct"
		}, "has none of: statement"},
		{"statements on one line", func(q *[]notes.Question) { (*q)[1].Question = "Statement I: A. Statement II: B." }, "quiz[1].question needs a line break"},
		{"direct question on one line", func(q *[]notes.Question) { (*q)[0].Question = "Where do T cells differentiate?" }, ""},
		{"three options", func(q *[]notes.Question) { (*q)[0].Options = (*q)[0].Options[:3] }, "exactly 4 options, it has 3"},
		{"five options", func(q *[]notes.Question) { (*q)[0].Options = append((*q)[0].Options, "Heart") }, "exactly 4 options, it has 5"},
		{"repeated option", func(q *[]notes.Question) { (*q)[0].Options[2] = " thymus" }, "options[2] repeats"},
		{"empty option", func(q *[]notes.Question) { (*q)[0].Options[2] = "" }, "options[2] is empty"},
		{"answer past the options", func(q *[]notes.Question) { (*q)[0].AnswerIndex = 4 }, "answer_index 4"},
		{"negative answer", func(q *[]notes.Question) { (*q)[0].AnswerIndex = -1 }, "answer_index -1"},
		{"no explanation", func(q *[]notes.Question) { (*q)[0].Explanation = "" }, "quiz[0].explanation is empty"},
		{"Hindi in an option", func(q *[]notes.Question) { (*q)[0].Options[0] = "थाइमस" }, "must be in English"},
		{"Hindi in an explanation", func(q *[]notes.Question) { (*q)[0].Explanation = "थाइमस" }, "must be in English"},
		{"time on no transcript line", func(q *[]notes.Question) { (*q)[0].StartSeconds = 41 }, "not the number on any transcript line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quiz := goodQuiz()
			tt.change(&quiz)
			problems := CheckQuiz(quiz, NEET.QuizTypes, 100, testLines)
			if tt.want == "" {
				if len(problems) != 0 {
					t.Errorf("unexpected problems: %v", problems)
				}
				return
			}
			if !strings.Contains(strings.Join(problems, "\n"), tt.want) {
				t.Errorf("problems %v, want one containing %q", problems, tt.want)
			}
		})
	}
}

func TestTargetCount(t *testing.T) {
	tests := []struct {
		name            string
		durationSeconds int
		wantCards       int
		wantQuestions   int
	}{
		{"short lecture hits the minimums", 10 * 60, 10, 5},
		{"68 minutes", 4119, 11, 6},
		{"2 hours", 2 * 60 * 60, 20, 10},
		{"4 hours hits the maximums", 4 * 60 * 60, 40, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := targetCount(tt.durationSeconds, secondsPerFlashcard, minFlashcards, maxFlashcards); got != tt.wantCards {
				t.Errorf("flashcards = %d, want %d", got, tt.wantCards)
			}
			if got := targetCount(tt.durationSeconds, secondsPerQuestion, minQuestions, maxQuestions); got != tt.wantQuestions {
				t.Errorf("questions = %d, want %d", got, tt.wantQuestions)
			}
		})
	}
}

func TestExamQuoteIgnoresSpacingAndCase(t *testing.T) {
	lines := []captions.Line{{StartSeconds: 0, Text: "This came in NEET"}, {StartSeconds: 40, Text: "in 2024 as well"}}
	note := goodNote()
	note.Sections[1].ExamMark = "Asked 2024"
	note.Sections[1].ExamQuote = "this came  in neet\nin 2024"
	if problems := CheckNote(note, lines); len(problems) != 0 {
		t.Errorf("unexpected problems: %v", problems)
	}
}
