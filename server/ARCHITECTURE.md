# Server

One Go project that builds two separate programs: the API and the worker. They never call each other; they meet only in the job tables in Postgres (Supabase once deployed). The API never calls the AI.

- `cmd/api`: answers the website. See [internal/api](internal/api/ARCHITECTURE.md).
- `cmd/worker`: runs until stopped, taking jobs and running the [pipeline](internal/pipeline/ARCHITECTURE.md). With `-url` it makes one note by hand, using local files and no database.
- `cmd/migrate`: applies [migrations](migrations/README.md). Run it before the other two.
- `internal/jobs`: the job tables, and the rules for claiming and saving. See [jobs](internal/jobs/ARCHITECTURE.md).

To run everything locally, see [deploy](../deploy/ARCHITECTURE.md).

The API has no login yet, so it binds to 127.0.0.1 by default. It must not be exposed publicly until step 6 (accounts and limits) is built: anyone who could reach it could spend AI calls.
