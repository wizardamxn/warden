# M08: Accounts, Revise, library and templates on the website

Status: built. The Go API for the same steps was built at the same time by another agent, so the website is tested against stand-ins that follow [docs/API.md](../../API.md).

## Goal

The website side of build steps 6, 5 and 7: students log in, see 3 notes a day, find their past notes, revise cards from lectures added more than 2 days ago, and get a quiz marked by their template. Anyone can open a sample lecture first.

## In this milestone

- Every API call sends `credentials: "include"`. The session is only the API's HttpOnly cookie; nothing is stored in the browser.
- Sign up (email, password, template with a plain line about each), log in, log out. Errors from the API's sentences, tied to their fields.
- Pages that need a login send logged-out visitors to `/login?next=...`, then back.
- Home logged out: what Warden does, sign up or log in, and a link to the sample. Logged in: the paste box and "1 of 3 notes today", a plain line at the limit, and the 429 and 503 said plainly.
- Library from `GET /v1/library`: newest first, linked, quiet status, the template only when it differs.
- Revise from `GET /v1/revise`: a pile of the existing index cards, each naming its lecture with a Watch link to its note at that moment (`/notes/<id>?t=<seconds>`). An empty state that says cards arrive 2 days after a lecture is added.
- Settings: the template, a line saying switching only affects new notes, and Log out.
- `/sample` from `GET /v1/sample` with no login.
- The answer sheet reads the note's `template`: NEET +4, −1, 0; General right out of total. The NCERT line only when present. Nothing on a General note says NEET.
- A quiet nav for logged-in students: Warden, Library, Revise, Settings.

## Not in this milestone

- Anything in `server/`.
- Password reset, email checks, changing email or password.
- A Revise algorithm, or saving which cards were seen.
- Dark mode and the phone layout (narrow widths must still work).

## Done when

- Playwright, with the API mocked to docs/API.md (cookies included), covers: signup with template, login, logout, redirect to login and back, the limit message and the 429, the 503, library and its empty state, revise and its empty state, a settings switch, the sample without login, General versus NEET scoring.
- The existing tests still pass, updated for login.
- Real Chrome screenshots of login, signup, home logged out, home logged in, library, revise, settings and a scored General sheet pass the slop test.
- Lint, type-check, production build and Playwright pass.
