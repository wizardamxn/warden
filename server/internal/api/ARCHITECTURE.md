# API

The HTTP API the website calls, on Go's standard `net/http`. It only reads and writes the job tables. It never calls the AI and never talks to the worker.

No login yet, so it binds to 127.0.0.1 by default and must not be exposed publicly until step 6 (accounts and limits) is built.

## Routes

```
GET  /healthz                     200 {"ok":true}
POST /v1/notes                    body {"url":"..."}; 200 if already finished, otherwise 202; 400 {"error":"..."} for a bad link
GET  /v1/notes/{video_id}         200, or 404 {"error":"..."}
POST /v1/notes/{video_id}/retry   202; re-queues only failed steps; 404 if unknown
```

Every successful response is:

```
{
  "video_id": "...",
  "status": "queued" | "running" | "done" | "partial" | "failed",
  "steps": [{ "name": "captions"|"notes"|"flashcards"|"quiz", "status": "pending"|"running"|"done"|"failed", "error": "" }],
  "note": <the note, in the note.json shape, or null>,
  "error": ""
}
```

- `note` appears as soon as the notes step is done, with flashcards and quiz as empty arrays until they're done, and `failed_steps` kept accurate. Arrays are never null.
- Every `error` is a plain sentence, shown to students as sent.
- "Finished" means `done`, `partial` or `failed`. A repeat POST doesn't re-queue failed steps, only retry does.
- Retry answers after the steps are re-queued, so its response already says `queued` or `running`.

## Requests

- Bodies are limited to 4 KB. An oversized or unreadable body is a 400.
- CORS allows only `WEB_ORIGIN`, plus `http://localhost:3000` in local use. A request from any other site is refused with a 403, because CORS alone only hides the answer: a page could still post and spend AI calls.
- Every request is logged with method, path, status, duration and a request ID, which also goes back in `X-Request-ID`.
