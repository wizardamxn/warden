# LLM

Turns a transcript into notes, flashcards and quizzes with Gemini.

## When Gemini refuses

- 503 (too busy): wait and try again, up to 5 attempts.
- 429 with a per-minute limit: the same, waiting for as long as Google's retry hint asks, at most 2 minutes.
- 429 with the daily quota used up: stop at once and fail the step. Waiting won't help. The error body's `quotaId` tells them apart, such as `GenerateRequestsPerDayPerProjectPerModel-FreeTier`.
- A 429 that doesn't say which limit is treated as per-minute.
- Each retry is logged with its HTTP status and which of these it was.
