# M03: Website with synced notes

Status: built. Results: [SUMMARY.md](SUMMARY.md).

This is step 3 of the build order in [docs/PLAN.md](../../PLAN.md).

## Goal

Paste a YouTube link on a real page and read the note beside the real video, with the highlight following playback.

## In this milestone

- Next.js with TypeScript in `apps/web`. No UI kit.
- Home page: paste a link, one button, waiting state, clear errors.
- Note page at `/notes/<video id>`: video left, notes right.
- YouTube IFrame Player API on `youtube-nocookie.com`. Click a timestamp to jump. The current time is read about twice a second to move the highlight.
- Embedding blocked: notes alone, timestamps open youtube.com at that second.
- Focus mode on the F key and a visible button.
- Red-pen exam marks, NCERT chapter name, mnemonics, glossary.

## Not in this milestone

- Flashcard and quiz screens (step 4), Revise tab, login, limits, Supabase, phone polish.

## Data for now

- The Go API isn't built, so a development-only route at `/api/dev/note` runs the worker and reads `server/.work/<id>/note.json`.
- All website calls go through `src/lib/api/notes.ts`, so swapping to the Go API changes one file.
- One saved real note lives in `apps/web/fixtures`, so testing spends no Gemini calls.

## Done when

- The saved note opens in a real browser, timestamps seek the real player, and the highlight moves.
- Focus mode and the embed-blocked fallback work.
- Lint, type-check, production build and Playwright tests pass.
