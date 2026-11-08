# Migrations

Database changes for Postgres (Supabase once deployed), one numbered file per change. Add a new file, don't edit one that has been applied.

From `server/`: `go run ./cmd/migrate`. It applies each file once, in order, and records it in `schema_migrations`. The files are built into the program.

- `001_jobs.sql`: jobs and job steps.
- `002_accounts.sql`: users, sessions and each student's library. Jobs become one per video and template; jobs made before are NEET jobs.

Every table has row level security on with no policies, so Supabase's own web API sees nothing. Our connection owns the tables and isn't affected.
