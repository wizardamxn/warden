# M07: Accounts, Revise and templates (backend)

Status: built and tested locally against Postgres, with fake Gemini and no YouTube downloads. The General prompts have never met the real API. Results: SUMMARY.md, once the main session adds it.

The Go side of build steps 5, 6 and 7 in [docs/PLAN.md](../../PLAN.md), built to the contract in [docs/API.md](../../API.md). The website side is built at the same time by another agent, from the same contract.

## Goal

A student signs up with a template, adds up to 3 lectures a day to their own library, opens only their own notes, and gets flashcards back in Revise once a lecture is 2 days old. Notes are made once per video and template.

## In this milestone

- Migration 002: users, sessions (token hash only), library, and jobs keyed by video and template, with old jobs becoming NEET. Row level security on the new tables.
- `internal/auth`: signup, login, logout, sessions in a 30-day cookie, argon2id passwords, login throttling by email and IP. `internal/users` merged into it.
- `internal/limits`: 3 new lectures per student per day, India time, and `TOTAL_DAILY_CAP` over new jobs. Both checked in the same transaction as the insert.
- The API routes and changes in docs/API.md: CORS with credentials and PATCH, login on every student route, library, revise, sample, me.
- Templates: NEET (today's prompts, unchanged) and General (any subject, direct and statement questions only). Prompt versions per template. The worker reads the template from the job. The note gains `template`, set by Go.
- `cmd/sample` to load a finished `note.json` as the sample lecture, and `SAMPLE_VIDEO_ID`.
- The worker's `-url` run gains `-template`, NEET by default.
- golang.org/x/crypto is the only new dependency.

## Not in this milestone

- Email check, password reset, Google sign-in: there's no email service.
- Signup throttling, a spaced-repetition algorithm, paging for Revise.
- Any change in `apps/`, deploying, going public.

## Decided

- Decided by the user: universal study helper; template picked at signup, stored on the user, read by the worker, changeable for new notes only; NEET and General first; 3 notes a day where every new lecture counts and Try again never does; a total cap with its number open; our own auth on Supabase's Postgres; Revise as a 2-day date filter; a simple library; results reused per video and template.
- Contract in docs/API.md, agreed with the website agent.

## Done when

- Every case in the build brief has a test: accounts, cookie and expiry, wrong password and throttling, the daily limit and its race, Try again and re-adding not counting, the total cap, one note per video and template, Revise's 2-day filter, per-student isolation, General quiz types, migration 002 on 001 data.
- `gofmt`, `go vet` and `go test -count=1 -v ./...` pass with the integration tests running, not skipping.
- No real Gemini calls and no YouTube downloads in any test.
