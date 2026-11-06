# Architecture

The whole system on one page: website, API, worker, Supabase and the tunnel.

## How a note gets made

The worker runs fixed steps in order: captions, notes, flashcards, quiz. No agent loop and no tool calls. The AI only returns JSON, Go checks it, and our website components draw it.

Every step is saved with its own marker, so a failed step never loses or regenerates the others. Details: [server/internal/jobs/ARCHITECTURE.md](../server/internal/jobs/ARCHITECTURE.md).

## How the parts talk

The website calls only the API ([server/internal/api/ARCHITECTURE.md](../server/internal/api/ARCHITECTURE.md)). The API adds jobs to Postgres, and the worker claims them, so the two programs never call each other. The API never calls the AI. The website checks the API every 2 seconds until a job ends.

A step stuck for about 10 minutes goes back to free. A job with no worker for about 30 minutes ends as failed, so a page never waits forever.
