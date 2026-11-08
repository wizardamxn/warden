# M08 summary

Built on 2026-11-08: the website side of accounts and the daily limit (step 6), Revise and the library (step 5), and templates (step 7), in `apps/web`. Tested with Playwright against stand-ins that follow docs/API.md, then checked once in real Chrome against the real Go API, with no Gemini calls.

## What was built

- Sign up (email, password, template with a plain line about each), log in, log out. Every API call sends `credentials: "include"`; the session is only the API's HttpOnly cookie, nothing is stored in the browser, and form values are never logged.
- Field errors sit under their field (409: the email). Any 401 sends the student to `/login?next=...` and back afterwards; `next` must be a path on this site.
- Home logged out: what Warden does, Sign up, Log in, and the sample lecture, linked only when `GET /v1/sample` has one. Logged in: the paste box and "1 of 3 notes today". At the limit the paste box gives way to a line saying the count starts again at midnight, India time. A 429 or 503 while pasting shows the API's sentence.
- Library: newest first, dates in the left margin like a note's timestamps, each title linked to its note ("Untitled lecture" until the captions are in), a quiet status word, the template only when it differs from the student's.
- Revise: the existing index card and keys, as a pile with the cards still to come askew underneath. Each card names its lecture and links "Watch 05:40" to `/notes/<id>?t=<seconds>`, which opens the note there. The empty state says cards arrive 2 days after a lecture is added.
- Settings: the template as OMR bubbles, a line saying switching only changes new notes, Save template, and Log out.
- `/sample`: the sample lecture in the same note screen, without a login.
- The answer sheet reads the note's `template`: NEET +4, −1 and 0; General a red tick or cross, scored right out of total. Nothing on a General note says NEET.
- A quiet top line for logged-in students: Warden, then Library, Revise and Settings, the current page underlined.

## Choices made while building

- User answers are read as `{"user": {...}}`, as the API sends them.
- "A date filter" for Revise is the server's 2-day filter; the page has no date picker.
- The template has no default at signup; the student must pick one.
- At the daily limit the paste box is hidden. Lectures already in the library open from Library.
- A finished note shows no status word in the library; only the others do.
- A signup 400 and a login 400 or 429 show under the whole form, because the API doesn't say which field.
- A note without `template` is treated as NEET.
- With no sample loaded, or the API unreachable, the home page leaves the sample line out instead of linking to a dead page.

## Real check, website against the real Go API

In real Chrome, with the API running and no worker, so no Gemini calls: the sample link on the logged-out home, the sample opening without a login, signup with General, an HttpOnly cookie that page scripts can't read, the empty library and Revise, switching the template to NEET and it staying after a reload, logout, the library sending a logged-out visitor to login, and logging back in returning to the library. No page errors.

## Still weak

- `/notes/<id>` opens the newest-template note for that video, so a Revise card or library row from the older template opens the newer note.
- The daily count shown is from page load; it doesn't change at midnight until the page reloads.
- The home page fetches the whole sample note to decide whether to show its link.
- The session cookie is SameSite=Lax, so the website and API must be the same site: both on 127.0.0.1 locally (localhost:3000 won't work), and on the API's registrable domain when deployed, not a default netlify.app or vercel.app address.

## Tested

- Playwright, 56 tests with the API mocked: signup with template and its HttpOnly cookie, signup and login errors, login 400 and 429, redirect to login and back (including a note page with `?t=`), refusing an outside `next`, logout, the settings switch and its 401, home logged out with and without a sample, the counter and nav, the limit message, 429 and 503 while pasting, the library and its empty state, Revise cards, keys and its empty state, Watch from Revise seeking the note, the sample without login and its 404, NEET and General scoring.
- Real Chrome screenshots at desktop and 390 px, checked against the slop test.
- Lint, type-check and the production build pass.

## Not tested

- Making a real note while logged in: it calls Gemini.
- The real YouTube player, screen readers, and contrast by measurement.
