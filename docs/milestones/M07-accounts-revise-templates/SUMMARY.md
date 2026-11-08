# M07 summary

Built on 2026-11-08: accounts, the daily limit and total cap, each student's library and Revise, and the NEET and General templates, in the Go backend. Tested against Postgres with a fake Gemini and no YouTube downloads.

## What works

- Signup, login and logout with our own accounts. Passwords are argon2id, the session cookie is HttpOnly and SameSite=Lax, and the database keeps only the token's SHA-256.
- Login throttling: 5 wrong passwords per email or 20 per address in 15 minutes, then 429.
- 3 new lectures a day per student, India time. Re-adding a lecture or Try again doesn't count. Two quick requests at 2 of 3 can't both pass.
- A total cap over new jobs a day, `TOTAL_DAILY_CAP`, safe against two students racing for its last place.
- Each student sees only their own library, notes and Revise cards. Tests prove another student gets a 404.
- Revise: flashcards from lectures added more than 2 days ago, oldest first.
- One note per video and template. Old jobs became NEET jobs, and nothing needed remaking.
- General template: notes, flashcards and a quiz of direct and statement questions only, with no NCERT, NEET or exam marks. NEET's prompts are unchanged.
- The sample lecture: `cmd/sample` loads a note.json, and `GET /v1/sample` serves it with no login.
- The worker's `-url` run still works, now with `-template`.

## Still weak

- No email check and no password reset: there's no email service.
- Signup isn't throttled, so accounts can be made in bulk. The total cap is the real limit, and its number isn't picked yet.
- Login throttling lives in memory: a restart forgets it.
- The website and API must be the same site for the cookie.
- The sample must be loaded again after a prompt change.

## Not tested

- The General prompts on the real Gemini API.
- A real Supabase, Go's race detector, and the website against this API.

## Before going public

Set `TOTAL_DAILY_CAP`, put the website and API on one domain, and decide whether signup needs a throttle.
