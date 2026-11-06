# M03 summary

Built on 2026-10-31: the website in `apps/web`, with the real YouTube player and synced notes. Tested on the saved note for `k7Zb1iOV-ys`, in real Chrome.

## What worked

- A pasted link opens the note at `/notes/<video id>`, with the video left and notes right.
- Timestamps seek the real player on `youtube-nocookie.com`, and the highlight moves with it, read about twice a second.
- The highlighter stroke sweeps under the playing section's heading and scrolls it into view.
- Focus mode works on the F key and a button. It's ignored while typing.
- Exam marks, the NCERT chapter, mnemonics and glossary show only when the note has them. Older notes without the newer fields still open.
- Tested with Playwright, using a stand-in for YouTube's script, so the tests don't depend on YouTube.

## What went wrong and was fixed

- The mockup's flex layout would stack the two columns below about 1376 px wide, which hits most laptops. It's a grid now, stacked only at 1200 px and under.
- Focus mode's shade covered the corners of the player, against YouTube's rules. The player now sits above it.
- A production build failed on the literal Go path, then warned that file reads would pull the whole project into the output. Both are fixed.

## Data for now

- `/api/dev/note` runs only in development. It checks `server/.work/<id>/note.json`, then `apps/web/fixtures/<id>.json`, and only then runs the worker.
- One run per video at a time, no retries, stopped after 30 minutes.
- If the worker exits with an error but wrote a note (flashcards or quiz failed), the note is used.

## Still weak

- The worker reports no steps yet, so the waiting screen shows a clock, not the growing glow. The glow needs real job progress.
- After a click inside the YouTube player, F goes to YouTube (fullscreen), because the page can't see key presses inside its iframe. Click the page first.
- `failed_steps` isn't shown yet. It only concerns flashcards and quiz, which arrive in step 4.
- If stopped at 30 minutes on Windows, the program `go run` started can keep running.

## Not tested

- A real worker run from the website: it uses Gemini calls. The route was tested with a stand-in for `go`.
- The fallback to `C:\Program Files\Go\bin\go.exe`: Go was on PATH here.
- A video that's really blocked from embedding. Error 150 was simulated.
- The phone layout, beyond checking that nothing overflows at 390 px.
