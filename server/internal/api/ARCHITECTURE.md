# API

The HTTP API the website calls, on Go's standard `net/http`. It reads and writes the job tables and the accounts. It never calls the AI and never talks to the worker. The contract agreed with the website is [docs/API.md](../../../docs/API.md).

It binds to 127.0.0.1 by default, which is where Cloudflare Tunnel reaches it. See [Before going public](#before-going-public).

## Routes

```
GET   /healthz                       200 {"ok":true}

POST  /v1/auth/signup   {email, password, template}   201 {"user": ...}, 400 bad input, 409 email taken
POST  /v1/auth/login    {email, password}             200 {"user": ...}, 401 wrong email or password, 429 too many tries
POST  /v1/auth/logout                                 204, ends the session and clears the cookie
GET   /v1/me                                          200 {"user": ...}
PATCH /v1/me            {template}                    200 {"user": ...}, 400 unknown template

POST  /v1/notes         {url}   200 if already finished, otherwise 202. 400 bad link, 429 daily limit, 503 total cap
GET   /v1/notes/{video_id}      200, or 404 if it isn't in the student's library
POST  /v1/notes/{video_id}/retry  202; re-queues only failed steps; 404 if it isn't in the student's library
GET   /v1/library               200 {"notes": [{video_id, title, channel, duration_seconds, template, added_at, status}]}
GET   /v1/revise                200 {"cards": [{video_id, title, template, added_at, question, answer, start_seconds}]}
GET   /v1/sample                200, the sample lecture's job, no login; 404 if none is set up
```

Every route under `/v1` except signup, login, logout and sample needs a login, and answers 401 `{"error": "Log in to continue."}` without one.

`user` is `{"id", "email", "template", "notes_today", "daily_limit"}`. The routes that answer with a job send:

```
{
  "video_id": "...",
  "status": "queued" | "running" | "done" | "partial" | "failed",
  "steps": [{ "name": "captions"|"notes"|"flashcards"|"quiz", "status": "pending"|"running"|"done"|"failed", "error": "" }],
  "note": <the note, in the note.json shape with "template", or null>,
  "error": ""
}
```

- `note` appears as soon as the notes step is done, with flashcards and quiz as empty arrays until they're done, and `failed_steps` kept accurate. Arrays are never null.
- `note.template` is `"neet"` or `"general"`, set by Go from the job, never by the AI. The website picks the quiz scoring from it.
- Every `error` is a plain sentence, shown to students as sent.
- "Finished" means `done`, `partial` or `failed`. A repeat POST doesn't re-queue failed steps, only retry does.
- Retry answers after the steps are re-queued, so its response already says `queued` or `running`.

## Students only see their own

- `POST /v1/notes` adds the lecture to the student's library under their current template. A note is made once per video and template, and shared by every student who adds it.
- `GET /v1/notes/{id}` and retry look the video up in the student's own library first, and use the template it was added with (the newest, if both). Another student's lecture is a 404, even with its video ID.
- Library and Revise only read the student's own library rows.
- Integration tests prove one student can't read another's library, notes or cards.

## Sessions

- Signup and login set `warden_session`: HttpOnly, SameSite=Lax, Path=/, 30 days, Secure except when the API is reached as 127.0.0.1 or localhost. It holds a random token; the database keeps only its SHA-256.
- SameSite=Lax means the website and the API must be the same site: `127.0.0.1:3000` with `127.0.0.1:8080` locally (not `localhost` for one and `127.0.0.1` for the other), and two subdomains of one domain when deployed.
- Logout deletes the session on the server too, so a copied cookie stops working.

## Requests

- Bodies are limited to 4 KB. An oversized or unreadable body is a 400.
- CORS allows only `WEB_ORIGIN`, plus `http://localhost:3000` in local use, with credentials and the PATCH method. A request from any other site is refused with a 403, because CORS alone only hides the answer: a page could still post and spend AI calls.
- Every request is logged with method, path, status, duration and a request ID, which also goes back in `X-Request-ID`. Bodies, cookies and headers are never logged.
- Login throttling uses the caller's address: `CF-Connecting-IP` when the request comes from this machine (the tunnel), otherwise the connection's own address.

## Before going public

Accounts, sessions, the daily limit and login throttling are built, so a stranger can no longer spend AI calls without an account. Still missing:

- `TOTAL_DAILY_CAP` is 0 (no cap) until a number is picked. With no email check, anyone can make many accounts, so the total cap is the real limit on the AI bill. Set it before going public.
- No email check and no password reset, because there's no email service. A student who forgets their password can't get back in.
- Signup isn't throttled, so accounts can be made in bulk.
- Login throttling lives in the API's memory: a restart forgets it, and it only works with one API process.
- The website and the API must share a domain for the cookie (see Sessions).
