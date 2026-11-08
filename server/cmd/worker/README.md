# Worker program

Starts the worker. Runs on the phone, takes the next job from Postgres, writes each step back. Ctrl+C stops it cleanly.

From `server/`:

- Run until stopped: `go run ./cmd/worker`
- One note by hand, with local files and no database: `go run ./cmd/worker -url <youtube link>`
- The same with the General template: `go run ./cmd/worker -url <youtube link> -template general`. NEET is the default.

The long-running worker reads each job's template from the database.
