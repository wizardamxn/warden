# M05: Website on the real API

Status: built. The Go API wasn't running while this was built, so it's tested against stand-ins.

## Goal

The website talks only to the Go API. Pasting a link opens the note page straight away, which shows real progress, then the note the moment it exists.

## In this milestone

- `src/lib/api/notes.ts` calls the Go API at `NEXT_PUBLIC_API_URL`, default `http://127.0.0.1:8080`.
- The development-only route `/api/dev/note` is gone, with everything only it needed.
- Paste a link: POST, then go to `/notes/<video id>`.
- The note page polls about every 2 seconds until `done`, `partial` or `failed`, and stops on leaving.
- The glow grows with each finished step. The words come only from real step statuses.
- The video stays on screen while waiting.
- The note appears as soon as it exists. Flashcards and quiz carry on behind it.
- Polling can't rebuild the player, interrupt the video, or reset the scroll or the highlight.
- Failed note: the plain error and Try again. Partial: the note stays usable, plus one quiet line and its Try again.
- Server down: a clear message and Try again, never an endless wait.

## Not in this milestone

- Flashcard and quiz screens (step 4), login, Revise, the phone layout, the sample lecture link.

## Done when

- Playwright, with the API mocked, covers: paste to note page, progress through real steps, the note arriving while flashcards run, polling not reloading the player, `partial` and `failed` with Try again, a 400 bad link, a 404 unknown video, the API down, and polling stopping.
- Real Chrome screenshots of each waiting step, a partial note and a failed note pass the slop test.
- Lint, type-check, production build and Playwright tests pass.
