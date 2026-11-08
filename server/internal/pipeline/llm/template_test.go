package llm

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"warden/internal/notes"
)

func goodGeneralQuiz() []notes.Question {
	quiz := goodQuiz()
	for i := range quiz {
		quiz[i].Type = General.QuizTypes[i%len(General.QuizTypes)]
	}
	return quiz
}

func TestTemplateFor(t *testing.T) {
	for _, name := range notes.Templates {
		if got, err := TemplateFor(name); err != nil || got.Name != name {
			t.Errorf("%s: got %q, %v", name, got.Name, err)
		}
	}
	if _, err := TemplateFor("jee"); err == nil {
		t.Error("an unknown template must be an error")
	}
}

func TestCheckGeneralQuiz(t *testing.T) {
	tests := []struct {
		name   string
		change func(q *[]notes.Question)
		want   string
	}{
		{"direct and statement", func(q *[]notes.Question) {}, ""},
		{"assertion-reason is not allowed", func(q *[]notes.Question) { (*q)[0].Type = "assertion-reason" }, `quiz[0].type "assertion-reason" must be one of direct, statement`},
		{"match is not allowed", func(q *[]notes.Question) { (*q)[2].Type = "match" }, `quiz[2].type "match"`},
		{"no statement question", func(q *[]notes.Question) {
			for i := range *q {
				(*q)[i].Type = "direct"
			}
		}, "has none of: statement"},
		{"statements on one line", func(q *[]notes.Question) { (*q)[1].Question = "Statement I: A. Statement II: B." }, "quiz[1].question needs a line break"},
		{"Hindi in a question", func(q *[]notes.Question) { (*q)[0].Question = "थाइमस क्या है" }, "must be in English"},
		{"time on no transcript line", func(q *[]notes.Question) { (*q)[0].StartSeconds = 41 }, "not the number on any transcript line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quiz := goodGeneralQuiz()
			tt.change(&quiz)
			problems := CheckQuiz(quiz, General.QuizTypes, 100, testLines)
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

func TestNEETQuizStillNeedsAllFourTypes(t *testing.T) {
	if problems := CheckQuiz(goodGeneralQuiz(), NEET.QuizTypes, 100, testLines); !strings.Contains(strings.Join(problems, "\n"), "has none of: assertion-reason, match") {
		t.Errorf("problems = %v", problems)
	}
}

var examWords = regexp.MustCompile(`(?i)\b(neet|ncert|pyq|exams?|assertion|match|medical)\b|exam_`)

func TestGeneralPromptsLeaveOutTheExam(t *testing.T) {
	for name, text := range map[string]string{
		"notes":            General.noteInstructions,
		"notes schema":     General.noteSchema,
		"flashcards":       General.flashcardInstructions,
		"quiz":             General.quizInstructions,
		"quiz schema":      General.quizSchema,
		"transcript rules": generalTranscriptRules,
	} {
		if found := examWords.FindString(text); found != "" {
			t.Errorf("General %s says %q", name, found)
		}
	}
	// The study steps keep the rules both templates share.
	for _, rule := range []string{"in English, in Latin letters", "start_seconds is the number in brackets"} {
		if !strings.Contains(General.flashcardInstructions, rule) || !strings.Contains(General.quizInstructions, rule) {
			t.Errorf("General prompts are missing %q", rule)
		}
	}
}

func TestMakeGeneralQuiz(t *testing.T) {
	client, requests := fakeGemini(t, reply(answerText(t, goodGeneralQuiz())))
	quiz, _, err := MakeQuiz(context.Background(), client, General, testVideo, testLines)
	if err != nil {
		t.Fatal(err)
	}
	if len(quiz) != minQuestions {
		t.Errorf("quiz = %d", len(quiz))
	}
	sent := (*requests)[0]
	if schema := string(sent.GenerationConfig.ResponseJSONSchema); !strings.Contains(schema, `"enum":["direct","statement"]`) {
		t.Errorf("schema = %s", schema)
	}
	if strings.Contains(sent.SystemInstruction.Parts[0].Text, "NEET") {
		t.Error("the General quiz was asked for with the NEET prompt")
	}
}

func TestMakeGeneralQuizSendsBackANEETType(t *testing.T) {
	bad := goodGeneralQuiz()
	bad[0].Type = "assertion-reason"
	client, requests := fakeGemini(t, reply(answerText(t, bad)), reply(answerText(t, goodGeneralQuiz())))
	if _, _, err := MakeQuiz(context.Background(), client, General, testVideo, testLines); err != nil {
		t.Fatal(err)
	}
	if fix := (*requests)[1].Contents[2].Parts[0].Text; !strings.Contains(fix, `quiz[0].type "assertion-reason"`) {
		t.Errorf("fix-up = %q", fix)
	}
}

func TestMakeGeneralNote(t *testing.T) {
	const answer = `{"tldr": "What a derivative is.", "key_points": ["Slope of a curve."],
		"sections": [{"start_seconds": 0, "title": "Derivatives", "bullets": ["The slope at a point."]}],
		"mnemonics": [], "glossary": []}`
	client, requests := fakeGemini(t, reply(answer))
	note, _, err := MakeNote(context.Background(), client, General, testVideo, testLines)
	if err != nil {
		t.Fatal(err)
	}
	if note.NCERTChapter != "" || note.VideoID != testVideo.ID || note.Sections[0].Title != "Derivatives" {
		t.Errorf("note = %+v", note)
	}
	if schema := string((*requests)[0].GenerationConfig.ResponseJSONSchema); strings.Contains(schema, "ncert_chapter") || strings.Contains(schema, "exam_mark") {
		t.Errorf("schema = %s", schema)
	}
}
