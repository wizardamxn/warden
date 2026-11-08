# API contract

What the website and the Go API agree on, for accounts (step 6), Revise (step 5) and templates (step 7). The routes from before stay as described in [server/internal/api/ARCHITECTURE.md](../server/internal/api/ARCHITECTURE.md), with the changes below.

JSON keys are snake_case. Every error is `{"error": "<a plain sentence for students>"}`. Arrays are never null.

## Sessions

- Login sets a cookie `warden_session`: HttpOnly, SameSite=Lax, Path=/, 30 days, Secure except on 127.0.0.1 or localhost. It holds a random token; the database keeps only its SHA-256.
- The website calls the API with `credentials: "include"`. The API answers CORS for `WEB_ORIGIN` with `Access-Control-Allow-Credentials: true`, and allows the PATCH method.
- A route that needs a login answers 401 `{"error": "Log in to continue."}` without one.

## Templates

- Two for now: `"neet"` (the prompts and NEET quiz mix built so far) and `"general"` (any subject: direct and statement questions only, plain score).
- A student picks one at signup and can change it later. It only affects notes made after the change.
- A note is made once per video and template. The same lecture under both templates is two notes.

## The user

```
user = { "id": 12, "email": "a@b.com", "template": "neet", "notes_today": 1, "daily_limit": 3 }
```

Wherever a route below answers `{user}`, the body is wrapped: `{"user": {...}}`.

`notes_today` counts lectures added to the student's library today, India time. Adding a lecture already in their library, or Try again, doesn't count.

## Account routes

```
POST  /v1/auth/signup   {email, password, template}  201 {user}       400 bad input, 409 email taken
POST  /v1/auth/login    {email, password}            200 {user}       400 unreadable body, 401 wrong email or password, 429 too many tries
POST  /v1/auth/logout                                204, clears the cookie
GET   /v1/me                                         200 {user}       401
PATCH /v1/me            {template}                   200 {user}       400 unknown template
```

- Email is trimmed and compared in lower case. Password is 8 to 128 characters.
- Login answers the same error whether the email or the password was wrong.
- The cookie is SameSite=Lax, so the website and the API must be the same site: both on 127.0.0.1 locally (localhost:3000 calling 127.0.0.1:8080 loses the cookie), and the website on the API's own domain when deployed, not a default netlify.app or vercel.app address.

## Notes routes, now per student

```
POST /v1/notes                   {url}   login needed. Adds the lecture to the student's library under their current template.
                                         200 or 202 with the job, as before.
                                         429 {"error": "You've made 3 notes today. More tomorrow."} at the daily limit.
                                         503 {"error": "Warden is at its limit for today. Try again tomorrow."} at the total cap.
GET  /v1/notes/{video_id}                login needed. The job for this video under the template it's in the student's library with
                                         (the newest, if more than one). 404 if it isn't in their library.
POST /v1/notes/{video_id}/retry          login needed, same lookup. Never counts toward the limit.
                                         Note: since the lookup takes the newest template, a library row or Revise card
                                         from an older template of the same video opens the newer note.
GET  /v1/sample                          no login. The sample lecture's finished note, same response shape. 404 if none is set up.
```

The job response gains one field inside `note`: `"template": "neet" | "general"`. The website uses it to pick the quiz scoring.

## Library and Revise

```
GET /v1/library   login needed.
                  200 {"notes": [{video_id, title, channel, duration_seconds, template, added_at, status}]}, newest first.
                  title is "" until the captions step is done. status is the job's status.

GET /v1/revise    login needed.
                  200 {"cards": [{video_id, title, template, added_at, question, answer, start_seconds}]}
                  Flashcards from library lectures added more than 2 days ago, oldest lecture first. No algorithm, just a date filter.
```

`added_at` is an RFC 3339 time.

## Limits

- 3 new lectures per student per day, India time (Asia/Kolkata).
- A total cap across all students: the `TOTAL_DAILY_CAP` setting, counting new jobs made today. 0 means no cap. The number isn't decided yet.
