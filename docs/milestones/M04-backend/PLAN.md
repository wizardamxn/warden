# M04: Real backend

Status: built and tested, except one real Gemini run to the end: Google refused every call on the day. Results: SUMMARY.md, once the main session adds it.

The job tables, API and worker that M03's development-only route stood in for. This backs everything built so far, ahead of steps 4 to 6 in [docs/PLAN.md](../../PLAN.md).

## Goal

Paste a link on the website, and the API and a long-running worker make the note through Postgres, surviving crashes without repeating paid AI work.

## In this milestone

- Postgres schema in numbered migrations: jobs and job steps.
- The worker becomes a long-running loop, claiming jobs with `FOR UPDATE SKIP LOCKED`. The one-off `-url` run stays, on the same pipeline.
- The API on Go's `net/http`: create a note, read it, retry failed steps.
- A logger on `log/slog`, in both programs and the pipeline.
- A used-up daily Gemini quota fails the step at once, with a clear message.
- Settings: `DATABASE_URL`, `API_ADDR`, `WEB_ORIGIN`, `LOG_LEVEL`.
- Local Postgres in Docker, and one script that runs everything.
- pgx is the only new Go dependency.

## Not in this milestone

- Login, the 3-a-day limit, the total cap.
- The phone and Cloudflare setup, deploying, a real Supabase project.

The API has no login, so it binds to 127.0.0.1 and can't be exposed publicly until step 6.

## Decided

- Two programs that never call each other, meeting only in the job tables. The API never calls the AI.
- Fixed pipeline, no agent loop.
- A step's result and its "done" marker are written together. A retry runs only steps without the marker.
- Notes, flashcards and quiz don't depend on each other.
- A step stuck as "running" for about 10 minutes goes back to free.
- Results are reused by video ID and prompt version.

## Done when

- One real lecture goes through the API, database and worker, in at most 8 Gemini calls.
- A second request for it makes no AI calls.
- A worker killed mid-job is replaced by a new one that repeats no saved steps.
- Integration tests pass on a real Postgres. `gofmt`, `go vet` and `go test -count=1 ./...` pass.
