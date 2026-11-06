package llm

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"warden/internal/notes"
	"warden/internal/pipeline/captions"
)

const (
	minQuoteWords  = 3
	optionCount    = 4
	maxSnapSeconds = 5
)

var yearPattern = regexp.MustCompile(`\b(19|20)\d{2}\b`)

// snapToLine moves a time the AI wrote back to the line being spoken then,
// because it often lands a second or two between lines. A time further off
// stays as it is, so the checks can catch it.
func snapToLine(seconds int, lines []captions.Line) int {
	next := sort.Search(len(lines), func(i int) bool { return lines[i].StartSeconds > seconds })
	if next > 0 && seconds-lines[next-1].StartSeconds <= maxSnapSeconds {
		return lines[next-1].StartSeconds
	}
	return seconds
}

// checker collects what's wrong with an answer from the AI.
type checker struct {
	problems   []string
	lineStarts map[int]bool
}

func newChecker(lines []captions.Line) *checker {
	starts := map[int]bool{}
	for _, line := range lines {
		starts[line.StartSeconds] = true
	}
	return &checker{lineStarts: starts}
}

func (c *checker) add(format string, args ...any) {
	c.problems = append(c.problems, fmt.Sprintf(format, args...))
}

func (c *checker) english(field, text string) {
	if hasDevanagari(text) {
		c.add("%s must be in English", field)
	}
}

func (c *checker) required(field, text string) {
	if strings.TrimSpace(text) == "" {
		c.add("%s is empty", field)
	}
	c.english(field, text)
}

// startTime makes sure a time is one of the transcript's own, so a link back
// to the video lands where something is really said.
func (c *checker) startTime(name string, seconds, durationSeconds int) {
	if seconds < 0 || seconds > durationSeconds {
		c.add("%s.start_seconds %d is outside the lecture, which is %d seconds long", name, seconds, durationSeconds)
	} else if !c.lineStarts[seconds] {
		c.add("%s.start_seconds %d is not the number on any transcript line", name, seconds)
	}
}

// count allows a fifth either side of the target, within the limits. Asking
// for exact numbers would lose a good answer over one extra card.
func (c *checker) count(field string, got, target, lowest, highest int) {
	low := max(lowest, target-target/5)
	high := min(highest, target+target/5)
	if got < low || got > high {
		c.add("%s has %d items, it needs %d", field, got, target)
	}
}

// CheckNote lists everything wrong with a note. The note comes from the AI,
// so nothing in it is trusted. An empty list means it passed.
func CheckNote(note notes.Note, lines []captions.Line) []string {
	c := newChecker(lines)

	c.english("ncert_chapter", note.NCERTChapter)
	if strings.ContainsAny(note.NCERTChapter, "0123456789") {
		c.add("ncert_chapter must be the chapter name only, with no numbers")
	}
	c.required("tldr", note.TLDR)
	if len(note.KeyPoints) == 0 {
		c.add("key_points is empty")
	}
	for i, point := range note.KeyPoints {
		c.required(fmt.Sprintf("key_points[%d]", i), point)
	}

	if len(note.Sections) == 0 {
		c.add("sections is empty")
	}
	spoken := spokenWords(lines)
	previousStart := -1
	for i, section := range note.Sections {
		name := fmt.Sprintf("sections[%d]", i)
		c.required(name+".title", section.Title)
		c.startTime(name, section.StartSeconds, note.DurationSeconds)
		if section.StartSeconds <= previousStart {
			c.add("%s.start_seconds %d must be after the section before it", name, section.StartSeconds)
		}
		previousStart = section.StartSeconds

		if len(section.Bullets) == 0 {
			c.add("%s.bullets is empty", name)
		}
		for j, bullet := range section.Bullets {
			c.required(fmt.Sprintf("%s.bullets[%d]", name, j), bullet)
		}
		for _, problem := range checkExamMark(section, spoken) {
			c.add("%s %s", name, problem)
		}
	}

	for i, mnemonic := range note.Mnemonics {
		c.required(fmt.Sprintf("mnemonics[%d].topic", i), mnemonic.Topic)
		c.required(fmt.Sprintf("mnemonics[%d].text", i), mnemonic.Text)
	}
	for i, term := range note.Glossary {
		c.required(fmt.Sprintf("glossary[%d].term", i), term.Term)
		c.required(fmt.Sprintf("glossary[%d].definition", i), term.Definition)
	}
	return c.problems
}

// CheckFlashcards lists everything wrong with a lecture's flashcards.
func CheckFlashcards(cards []notes.Flashcard, durationSeconds int, lines []captions.Line) []string {
	c := newChecker(lines)
	c.count("flashcards", len(cards), targetCount(durationSeconds, secondsPerFlashcard, minFlashcards, maxFlashcards), minFlashcards, maxFlashcards)
	for i, card := range cards {
		name := fmt.Sprintf("flashcards[%d]", i)
		c.required(name+".question", card.Question)
		c.required(name+".answer", card.Answer)
		c.startTime(name, card.StartSeconds, durationSeconds)
	}
	return c.problems
}

// CheckQuiz lists everything wrong with a lecture's quiz.
func CheckQuiz(quiz []notes.Question, durationSeconds int, lines []captions.Line) []string {
	c := newChecker(lines)
	c.count("quiz", len(quiz), targetCount(durationSeconds, secondsPerQuestion, minQuestions, maxQuestions), minQuestions, maxQuestions)

	missing := slices.Clone(quizTypes)
	for i, question := range quiz {
		name := fmt.Sprintf("quiz[%d]", i)
		if !slices.Contains(quizTypes, question.Type) {
			c.add("%s.type %q must be one of %s", name, question.Type, strings.Join(quizTypes, ", "))
		}
		missing = slices.DeleteFunc(missing, func(t string) bool { return t == question.Type })
		c.required(name+".question", question.Question)
		if question.Type != "direct" && !strings.Contains(question.Question, "\n") {
			c.add("%s.question needs a line break between its statements or list items", name)
		}
		c.required(name+".explanation", question.Explanation)
		c.startTime(name, question.StartSeconds, durationSeconds)

		if len(question.Options) != optionCount {
			c.add("%s needs exactly %d options, it has %d", name, optionCount, len(question.Options))
		}
		seen := map[string]bool{}
		for j, option := range question.Options {
			c.required(fmt.Sprintf("%s.options[%d]", name, j), option)
			if seen[squeeze(option)] {
				c.add("%s.options[%d] repeats another option", name, j)
			}
			seen[squeeze(option)] = true
		}
		if question.AnswerIndex < 0 || question.AnswerIndex >= optionCount {
			c.add("%s.answer_index %d must be 0 to %d", name, question.AnswerIndex, optionCount-1)
		}
	}
	if len(missing) > 0 {
		c.add("quiz needs at least one question of each type, it has none of: %s", strings.Join(missing, ", "))
	}
	return c.problems
}

// checkExamMark makes sure a mark is backed by what the teacher really said.
// Students will trust "Asked 2024", so an invented one must never get through.
func checkExamMark(section notes.Section, spoken string) []string {
	mark := strings.TrimSpace(section.ExamMark)
	quote := squeeze(section.ExamQuote)
	if mark == "" && quote == "" {
		return nil
	}
	if mark == "" || quote == "" {
		return []string{"needs both exam_mark and exam_quote, or neither"}
	}

	var problems []string
	if hasDevanagari(mark) {
		problems = append(problems, "exam_mark must be in English")
	}
	if len(strings.Fields(quote)) < minQuoteWords {
		problems = append(problems, "exam_quote is too short to check")
	}
	if !strings.Contains(spoken, quote) {
		problems = append(problems, fmt.Sprintf("exam_quote %q is not in the transcript, copy the exact words", section.ExamQuote))
	}
	for _, year := range yearPattern.FindAllString(mark, -1) {
		if !strings.Contains(quote, year) {
			problems = append(problems, "exam_mark names "+year+" but exam_quote doesn't say it")
		}
	}
	return problems
}

// withoutBadExamMarks clears every mark that fails its check. ok is true if
// the note then passes everything else.
func withoutBadExamMarks(note notes.Note, lines []captions.Line) (notes.Note, bool) {
	spoken := spokenWords(lines)
	for i := range note.Sections {
		if len(checkExamMark(note.Sections[i], spoken)) > 0 {
			note.Sections[i].ExamMark = ""
			note.Sections[i].ExamQuote = ""
		}
	}
	return note, len(CheckNote(note, lines)) == 0
}

func hasDevanagari(text string) bool {
	return strings.IndexFunc(text, func(r rune) bool { return unicode.Is(unicode.Devanagari, r) }) >= 0
}

// squeeze makes text comparable: lower case, single spaces.
func squeeze(text string) string {
	return strings.ToLower(strings.Join(strings.Fields(text), " "))
}

// spokenWords joins the transcript without its times, so a quote can run across lines.
func spokenWords(lines []captions.Line) string {
	var words strings.Builder
	for _, line := range lines {
		words.WriteString(line.Text)
		words.WriteByte(' ')
	}
	return squeeze(words.String())
}
