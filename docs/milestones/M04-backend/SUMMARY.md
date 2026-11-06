# M04 summary

Built on 2026-10-31: the job tables, the API and a long-running worker. Tested on `G_em1hHPnXY` (6:25). The first real run was blocked by Google, and a second one finished, see below.

## What worked

- A link goes through the API into Postgres, and the worker claims it and runs the steps. Real captions took 12 seconds.
- A worker killed hard mid-job is replaced by a new one that repeats no saved step. Tested twice, with the real 10-minute timeout. After a kill during quiz, only quiz ran again.
- Ctrl+C stops a step, puts it back to free and lets go of the job, so the next worker starts at once.
- A second request for a finished video answered in 56 ms with no AI calls.
- Try again runs only failed steps and answers with the full response.
- A job with no worker left still ends: polling alone frees a stuck step at about 10 minutes and ends the job at about 30.
- Busy Gemini: 5 attempts over 2m39s, then a plain sentence for the student.

## What went wrong and was fixed

- Saves would have failed on Supabase's transaction pooler, because bytes were sent as bytea. Now sent as text. Found by running the tests in that mode.
- A test that skips looks like a pass. One TLS run "passed" in 0.2 seconds because the database was down. The skip message now gives the reason.
- Stuck steps were only freed when a worker claimed the job, so with no worker the website would poll forever. The API's reads now do it too.

## Real run

First attempt, while building:

- Google refused every notes call with 503 "high demand".
- 7 of the 8 allowed calls used: 1 cancelled by a Ctrl+C test, 5 refused, 1 in flight at a hard kill. None answered. Recorded cost: $0.
- The kill tests then used a local stand-in replaying this video's saved real answers. Everything else in them was real.

Second attempt, after building, through Docker's Postgres, the real API, worker and Gemini:

- Link to partial note in 70 seconds: captions 12 s, notes 7 s, flashcards 25 s, quiz failed after 26 s.
- The quiz failed because Gemini wrote times between transcript lines. Times up to 5 seconds off now snap back to the line being spoken, instead of costing a fix-up.
- A real Try again ran only the quiz, in 17 seconds, and the job ended `done`.
- A repeat request for the finished video answered in 9 ms, with no AI calls.
- Cost at paid prices: about ₹0.93 for the first run and ₹0.42 for the retry.
- The real website, opened in real Chrome, showed the note from the real API with no console errors.

## Still weak

- With a queue over 30 minutes long, waiting jobs fail too. Try again works.
- Try again on a video that can never work fails the same way again. It costs no AI calls.
- A new prompt version overwrites the old result when that lecture is next requested.

## Not tested

- A real Supabase project or pooler. TLS and the pooler's mode were tested on local Postgres.
- A real 429 from Google. The daily and per-minute handling is tested on a sample body.
- Go's race detector, which needs cgo.

## Before going public

The API has no login. It stays on 127.0.0.1 until step 6.
