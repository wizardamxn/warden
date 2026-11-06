# Core

Settings, database connection and logging that everything else uses.

## Settings

Read from `server/.env`. A real environment variable wins over the file.

| Setting | Default | Used by |
|---|---|---|
| `GEMINI_API_KEY` | none, required | worker |
| `DATABASE_URL` | none, required | API, worker, migrate |
| `API_ADDR` | `127.0.0.1:8080` | API |
| `WEB_ORIGIN` | `http://127.0.0.1:3000` | API. With the default, `http://localhost:3000` is allowed too |
| `LOG_LEVEL` | `info` | API, worker: `debug`, `info`, `warn` or `error` |

A program won't start without what it needs. The error names the setting and never prints a value.

## Database

- `OpenDB` connects with the driver pgx. TLS and other options come from the URL.
- Supabase, direct or session pooler: add `?sslmode=require`.
- Supabase transaction pooler (port 6543): add `?sslmode=require&default_query_exec_mode=simple_protocol`, because that pooler can't keep prepared statements.

## Logging

- Go's `log/slog`, as readable `key=value` text on stderr.
- `core.LogWith(ctx, ...)` adds fields, and `core.Log(ctx)` logs with them. That's how every line about a job carries `video_id`, `job_id` and `step`, and every request line carries `request_id`, without passing them by hand.
- Levels: `debug` for each Gemini call and skipped steps, `info` for normal progress, `warn` for retries, failed checks and partial jobs, `error` for failed steps and requests.
- Never logged: the API key, database URLs or passwords, full transcripts, full prompts.
