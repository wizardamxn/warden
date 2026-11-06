package llm

import (
	"context"
	"fmt"
	"math/rand/v2"

	"warden/internal/notes"
	"warden/internal/pipeline/captions"
)

// Flashcards and quiz questions scale with the lecture's length.
const (
	secondsPerFlashcard = 6 * 60
	minFlashcards       = 10
	maxFlashcards       = 40

	secondsPerQuestion = 12 * 60
	minQuestions       = 5
	maxQuestions       = 20
)

// quizTypes is the NEET mix. A quiz needs all of them.
var quizTypes = []string{"direct", "statement", "assertion-reason", "match"}

// worthTesting is what to pick first, for flashcards and quiz alike.
const worthTesting = `- Choose what's worth testing, in this order: first everything the teacher says came in NEET or calls a previous year question or PYQ, then what the teacher calls important, then exceptions and things students mix up, then definitions and lists. Finish each group before starting the next. Use the whole lecture, not only the start.
- start_seconds is the number in brackets on the line where the teacher states that exact fact, copied exactly. Not the line where the topic first comes up.`

const flashcardInstructions = `You write flashcards for students preparing for NEET, India's medical entrance exam.

` + transcriptRules + `

Rules:
` + contentRules + `
` + worthTesting + `
- Each card tests one fact. No two cards test the same fact.
- Each card must make sense days later with no lecture around it. Name the topic in the question, and never write "the teacher", "this lecture", "as discussed" or "above".
- answer: a word, a phrase or one short sentence.`

const flashcardSchema = `{
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "question": { "type": "string" },
      "answer": { "type": "string" },
      "start_seconds": { "type": "integer" }
    },
    "required": ["question", "answer", "start_seconds"]
  }
}`

const quizInstructions = `You write multiple-choice questions in the style of NEET, India's medical entrance exam.

` + transcriptRules + `

Rules:
` + contentRules + `
` + worthTesting + `
- Use all four types at least once:
  - "direct": a plain question.
  - "statement": two or more statements, written "Statement I: ..." and "Statement II: ...", and options saying which are correct.
  - "assertion-reason": written "Assertion (A): ..." and "Reason (R): ...", with exactly these options: "Both A and R are true and R is the correct explanation of A", "Both A and R are true but R is not the correct explanation of A", "A is true but R is false", "A is false but R is true". R is only the correct explanation if R is the actual cause of A. If R is true and merely related to A, the second option is correct.
  - "match": "List I" with items A, B, C, D and "List II" with items I, II, III, IV, both inside the question. Each option is a full pairing such as "A-II, B-I, C-IV, D-III". Shuffle List II so the correct pairing is never A-I, B-II, C-III, D-IV.
- In every type except "direct", write each statement, each list heading and each list item on its own line, using a line break character.
- options: exactly 4 different options with exactly one correct, and no letters or numbers in front of them. answer_index is the position of the correct one, counting from 0. Across the quiz, use every position from 0 to 3, about equally.
- Every option must be a real answer to the question asked, so the question and its correct option read as one true sentence.
- Wrong options come from what students confuse: a related term, a neighbouring scientist or date, a part that sounds alike. Never use a silly option.
- explanation: always two parts. First why the answer is right, then why the main wrong option is wrong, naming that option. Name options by what they say, never by letter or position.
- Each question and explanation must make sense days later with no lecture around it. Name the topic in the question, and never write "the teacher", "the lecture", "this lecture", "mentioned", "as discussed" or "above".`

const quizSchema = `{
  "type": "array",
  "items": {
    "type": "object",
    "properties": {
      "type": { "type": "string", "enum": ["direct", "statement", "assertion-reason", "match"] },
      "question": { "type": "string" },
      "options": { "type": "array", "items": { "type": "string" }, "minItems": 4, "maxItems": 4 },
      "answer_index": { "type": "integer" },
      "explanation": { "type": "string" },
      "start_seconds": { "type": "integer" }
    },
    "required": ["type", "question", "options", "answer_index", "explanation", "start_seconds"]
  }
}`

// targetCount scales a count with the lecture's length, within its limits.
func targetCount(durationSeconds, secondsPerItem, lowest, highest int) int {
	return min(max((durationSeconds+secondsPerItem/2)/secondsPerItem, lowest), highest)
}

// MakeFlashcards asks Gemini for flashcards and checks them.
func MakeFlashcards(ctx context.Context, client *Client, video captions.Video, lines []captions.Line) ([]notes.Flashcard, Usage, error) {
	target := targetCount(video.DurationSeconds, secondsPerFlashcard, minFlashcards, maxFlashcards)
	return ask(ctx, client, request[[]notes.Flashcard]{
		step:         "flashcards",
		instructions: flashcardInstructions,
		schema:       flashcardSchema,
		prompt:       fmt.Sprintf("Write %d flashcards.\n\n%s", target, lecturePrompt(video, lines)),
		check: func(cards *[]notes.Flashcard) []string {
			for i := range *cards {
				(*cards)[i].StartSeconds = snapToLine((*cards)[i].StartSeconds, lines)
			}
			return CheckFlashcards(*cards, video.DurationSeconds, lines)
		},
	})
}

// MakeQuiz asks Gemini for quiz questions and checks them.
func MakeQuiz(ctx context.Context, client *Client, video captions.Video, lines []captions.Line) ([]notes.Question, Usage, error) {
	target := targetCount(video.DurationSeconds, secondsPerQuestion, minQuestions, maxQuestions)
	quiz, usage, err := ask(ctx, client, request[[]notes.Question]{
		step:         "quiz",
		instructions: quizInstructions,
		schema:       quizSchema,
		prompt:       fmt.Sprintf("Write %d questions.\n\n%s", target, lecturePrompt(video, lines)),
		check: func(quiz *[]notes.Question) []string {
			for i := range *quiz {
				(*quiz)[i].StartSeconds = snapToLine((*quiz)[i].StartSeconds, lines)
			}
			return CheckQuiz(*quiz, video.DurationSeconds, lines)
		},
	})
	if err == nil {
		shuffleOptions(quiz)
	}
	return quiz, usage, err
}

// shuffleOptions moves the right answer to a random place, because Gemini
// tends to put it first. Statement and assertion-reason options keep the fixed
// order NEET prints them in.
func shuffleOptions(quiz []notes.Question) {
	for i := range quiz {
		q := &quiz[i]
		if q.Type != "direct" && q.Type != "match" {
			continue
		}
		rand.Shuffle(len(q.Options), func(a, b int) {
			q.Options[a], q.Options[b] = q.Options[b], q.Options[a]
			switch q.AnswerIndex {
			case a:
				q.AnswerIndex = b
			case b:
				q.AnswerIndex = a
			}
		})
	}
}
