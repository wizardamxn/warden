# M02: NEET study material

Status: built and tested on one real lecture. Results: [SUMMARY.md](SUMMARY.md).

This is step 2 of the build order in [docs/PLAN.md](../../PLAN.md).

## Goal

One worker run gives the full `note.json`: a note that covers the whole lecture, NCERT chapter name, mnemonics, flashcards and a NEET style quiz. Every card and question links back to its moment in the video.

## In this milestone

- Notes, flashcards and quiz are three separate Gemini calls. Each reads the whole transcript and is saved in its own file.
- If one fails the others stay saved, and a re-run only runs what's missing.
- Every check failure gets one fix-up attempt with the problem list, then that step stops. All three steps share this loop.
- Fix the thin notes from M01.

## Not in this milestone

- Website, jobs table, Supabase, login.
- Past NEET questions, or any NCERT text.
- Shuffling quiz options in Go.

## Decided

- Fixed pipeline. No agent loop, no tool calls. The AI returns JSON, Go checks it, our website draws it.
- Everything the student sees is in English with English NCERT terms. `exam_quote` is the one exception.
- NCERT references: chapter name only.
- Flashcards: about 1 per 6 minutes, min 10, max 40. One fact per card.
- Quiz: about 1 question per 12 minutes, min 5, max 20. Direct, statement, assertion-reason and match. Exactly 4 distinct options, one correct.
- Wrong options come from what students confuse. The explanation says why the answer is right and why the main wrong option is wrong.
- What's worth testing, in order: what the teacher says came in NEET, what they call important, exceptions and mix-ups, definitions and lists.
- Mnemonics: the teacher's own first. Gemini may make one up if it's accurate to the lecture.

## Note shape

```
video_id, title, channel, duration_seconds, source
ncert_chapter (empty if unclear)
tldr
key_points: [text]
sections: [{ start_seconds, title, bullets: [text], exam_mark, exam_quote }]
mnemonics: [{ topic, text }]
glossary: [{ term, definition }]
flashcards: [{ question, answer, start_seconds }]
quiz: [{ type, question, options: [4 texts], answer_index, explanation, start_seconds }]
failed_steps: [text]
```

- `type` is "direct", "statement", "assertion-reason" or "match".
- `video_id` comes from our own data, never from the AI.
- Arrays are never null.
- `question` can hold line breaks, for statements and match lists.

## When a step fails

- Flashcards or quiz: the note is still written, with an empty array and the step's name in `failed_steps`. The worker exits non-zero with a clear message.
- Notes: nothing to show, so the worker stops with the error. Flashcards and quiz still run first, so their results are saved for the retry.

## Where the code goes

- `server/internal/notes/`: the new fields.
- `server/internal/pipeline/llm/note.go`: the notes prompt, and the rules all three prompts share.
- `server/internal/pipeline/llm/study.go`: flashcard and quiz prompts, and the counts.
- `server/internal/pipeline/llm/gemini.go`: `ask`, the shared fix-up loop.
- `server/internal/pipeline/llm/check.go`: the checks.
- `server/internal/pipeline/pipeline.go`: the steps in order.

## Budget

- One lecture only, `k7Zb1iOV-ys`, using its saved transcript.
- At most 12 real Gemini calls.
- Stop on a rate limit. No test calls the real API.

## Done when

- The same lecture gets a note that covers it properly, with cards and a quiz.
- Timestamps, biology and whether questions stay inside the lecture are spot-checked by reading them against the transcript.
- `SUMMARY.md` says honestly what's weak.
