# Migrations

Database changes for Postgres (Supabase once deployed), one numbered file per change. Add a new file, don't edit one that has been applied.

From `server/`: `go run ./cmd/migrate`. It applies each file once, in order, and records it in `schema_migrations`. The files are built into the program.
