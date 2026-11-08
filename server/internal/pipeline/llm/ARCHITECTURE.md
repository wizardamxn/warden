# LLM

Turns a transcript into notes, flashcards and quizzes with Gemini.

## Templates

A template is the three prompts, their schemas, their versions and the quiz's question types.

- `neet`: the prompts built in M02, unchanged, at versions v4, v2 and v3. NCERT chapter, exam marks, and a quiz needing direct, statement, assertion-reason and match questions.
- `general`: any subject. No NCERT, no NEET, no exam marks. The quiz mixes only direct and statement questions. Versions start at `general-v1`.
- Both keep the same English-only, timestamp and option checks. `CheckQuiz` takes the template's question types: each is needed, and no other is allowed.
- Direct and match options are shuffled for both, as before.
- The General prompts have never been run against the real Gemini API. Only fakes have answered them.

## When Gemini refuses

- 503 (too busy): wait and try again, up to 5 attempts.
- 429 with a per-minute limit: the same, waiting for as long as Google's retry hint asks, at most 2 minutes.
- 429 with the daily quota used up: stop at once and fail the step. Waiting won't help. The error body's `quotaId` tells them apart, such as `GenerateRequestsPerDayPerProjectPerModel-FreeTier`.
- A 429 that doesn't say which limit is treated as per-minute.
- Each retry is logged with its HTTP status and which of these it was.
