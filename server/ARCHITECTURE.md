# Server

One Go project that builds two separate programs: the API and the worker. They never call each other; they meet only in the job tables in Postgres (Supabase once deployed). The API never calls the AI.

- `cmd/api`: answers the website. See [internal/api](internal/api/ARCHITECTURE.md).
- `cmd/worker`: runs until stopped, taking jobs and running the [pipeline](internal/pipeline/ARCHITECTURE.md). With `-url` it makes one note by hand, using local files and no database.
- `cmd/migrate`: applies [migrations](migrations/README.md). Run it before the other two.
- `cmd/sample`: stores a finished note as the sample lecture. See [cmd/sample](cmd/sample/README.md).
- `internal/jobs`: the job tables, each student's library and Revise, and the rules for claiming and saving. See [jobs](internal/jobs/ARCHITECTURE.md).
- `internal/auth`: accounts, passwords and sessions. See [auth](internal/auth/ARCHITECTURE.md).
- `internal/limits`: the daily limit and the total cap. See [limits](internal/limits/ARCHITECTURE.md).

To run everything locally, see [deploy](../deploy/ARCHITECTURE.md).

The API now has accounts and limits, but it still binds to 127.0.0.1 by default and isn't ready to be public yet. What's missing is listed in [internal/api](internal/api/ARCHITECTURE.md#before-going-public).
