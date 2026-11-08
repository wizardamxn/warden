# Website

The Next.js website, hosted on Netlify or Vercel. It talks only to the API at api.<domain>.

The API address is the `NEXT_PUBLIC_API_URL` setting, default `http://127.0.0.1:8080`. It's fixed when the website is built, so a deployed build must set it. All calls go through `src/lib/api/client.ts`, with one file per area (`notes.ts`, `account.ts`). The contract is [docs/API.md](../../docs/API.md).

## Accounts

- The cookie is SameSite=Lax, so the website and the API must be the same site. Locally both run on 127.0.0.1: the website at `http://127.0.0.1:3000` (`npm run dev` already binds there) and the API at `http://127.0.0.1:8080`. Opening the website at localhost:3000 makes it cross-site to 127.0.0.1:8080, and the cookie isn't sent. Deployed, the website must be on the API's domain (`<domain>` with `api.<domain>`), not a default netlify.app or vercel.app address.
- The session is only the API's HttpOnly `warden_session` cookie. Every call sends `credentials: "include"`. The website never stores a token, in localStorage or anywhere else.
- Pages ask `GET /v1/me` who's logged in (`useMe` in `src/lib/auth/session.ts`). The cookie belongs to the API's host, so the website's own server can't read it; every check happens in the browser.
- A page that needs a login sends a logged-out visitor (any 401) to `/login?next=<where they were going>`, and login sends them back. `next` must be a path on this site.
- Signup asks for email, password and template. Errors come from the API's plain sentences and sit under the field they're about (409: the email). Form values are never logged.
- Home logged out: what Warden does, sign up or log in, and the sample lecture, linked only once `GET /v1/sample` has one (404: no sample loaded, so no link). Logged in: the paste box and "1 of 3 notes today", then what's on the desk: how many cards wait in Revise with a link to start, and the last 3 lectures (with "All of your library" when there are more). Either part just stays away when it's empty or can't be loaded. At the limit the paste box gives way to a line saying it resets tomorrow, with the desk under it. A 429 or 503 while pasting shows the API's sentence.
- Settings: the template, saved with `PATCH /v1/me`, and Log out. Switching only changes notes made from then on.
- Logged-in pages share a quiet top line: Warden, then Library, Revise and Settings at the end, the current page underlined.

## Library, Revise and the sample

- Library: `GET /v1/library`, newest first. A lecture whose captions aren't in yet has `title` "" and shows as "Untitled lecture". Dates hang in the left margin like a note's timestamps, then the lecture's still from YouTube (`i.ytimg.com`, no key), then the title. Each title links to its note, and so does the still, for the mouse only (keyboards and screen readers get the title); status is a quiet word ("Being made", nothing when done), and the template only shows when it differs from the student's current one.
- Empty library or Revise: says what to do next, and offers the sample lecture when one is loaded. An empty Revise reads the library to say when the next cards arrive (2 days after the soonest new lecture).
- Revise: `GET /v1/revise`, a pile of index cards (the same `Flashcards` component and keys as a note), the cards still to come lying askew underneath. Each card names its lecture and links "Watch 05:40" to `/notes/<id>?t=<seconds>`, which opens the note there.
- The sample: `/sample` shows `GET /v1/sample` in the same note screen, with no login. Try again only looks again; it never asks the API to retry the sample.

## Templates

- A note carries `template` (`neet` or `general`; older notes without it are NEET).
- NEET: the answer sheet is marked +4, −1 and 0. General: a tick or cross, and the score is right out of total. The instruction and score lines change to match, and nothing on a General note says NEET.
- The NCERT chapter line only shows when the note has one.

## Making a note

- Pasting a link sends it to the API, then goes straight to `/notes/<video id>`.
- That page asks the API for the note about every 2 seconds until the status is `done`, `partial` or `failed`, and stops when you leave.
- If the server misses three checks in a row, the page says so, stops asking, and offers Try again.
- What the page says it's doing comes only from the real step statuses. The glow grows with each finished step.
- The note shows as soon as it exists, while flashcards and quiz carry on behind it.
- One component (`NoteView`) stays on screen from the first wait to the finished note, so polling can't rebuild the player or lose the scroll.
- Note text comes from an AI, so it's always shown as plain text.

## Testing yourself

- Once the note exists, a quiet nav switches the right column between Notes, Flashcards and Answer sheet. The video stays on the left and is never rebuilt.
- All three views stay mounted and are only hidden, so a half-filled sheet or a card's place survives switching.
- The end of the notes leads into the flashcards, and the last card leads into the answer sheet.
- "Watch 05:40" on a card or question seeks the same player and switches back to Notes at that moment. If the video can't be embedded, it opens youtube.com at that second instead.
- Flashcards: one index card at a time. Click, Space or Enter turns it over; the arrow keys move between cards. Keys are ignored while typing.
- Answer sheet: the whole quiz as an OMR sheet, scored by the note's template (above). Each question is a native radio group. Submit once, then each question shows its mark, the right answer, the explanation and its Watch link. Nothing is saved.
- Right and wrong are always told in words as well as colour ("Right answer", "Your answer, wrong", "+4 right").
- Match questions show List I and List II side by side when the text holds both lists that pair up; otherwise the question shows as its own lines.
- While flashcards or quiz are still being made, their view says so from the real step status. A failed step says which part couldn't be made, with Try again.
- Quiz and flashcard text comes from an AI too, so it's plain text only.

## Theme

- Light and dark. Colours, fonts, spacing, the glow and the highlighter: [docs/DESIGN.md](../../docs/DESIGN.md).
- Every colour has a name ("background", "text", "accent", ...) defined in one place. Screens use only the names, never raw colour values.
- Dark is one more set of values for the same names (`globals.css`), so no screen knows which theme is on. It follows the device unless Settings picks Light or Dark. That choice is kept in this browser only (localStorage, the one thing the website keeps there), and a small script in `<head>` applies it before the page draws, so the other theme never flashes.

## Layout

- Desktop first: video on the left, notes on the right.
- Phone, later: video sticks to the top, notes scroll under it. Every page must still work at narrow widths.
- Notes column stays narrow, about 60–75 characters per line, for easy reading.

## Embedded video

- YouTube's IFrame Player API, privacy-enhanced (`youtube-nocookie.com`). Free, no API key, YouTube streams the video.
- Our page controls it: jump to a time (timestamps), read the current time (live highlight, checked about twice a second), play/pause/ready events.
- Don't hide or cover YouTube's player, branding or ads (their rules).
- Phones block autoplay with sound: the user taps play once, then timestamp jumps work.
- If the owner blocked embedding (player error 101 or 150): show the notes on their own, and make each timestamp open youtube.com at that second in a new tab.

## Motion

- Short (about 0.15–0.4 s) and only to show that something changed.
- Turned off when the device asks for less motion. Nothing must depend on an animation to work.
- The quiz's one celebration: after submitting, the light moves from the video to the score and the highlighter sweeps under it, once per submit. The score counts up to itself, and question by question the right answer is swept with the highlighter and the red pen stamps the mark.
- Moving between pages: React's `<ViewTransition>` (`components/ui/Arrive.tsx`, around every page). The old page fades out fast, the new one rises 12 px into place; the top line stays still. A lecture's still in the library or on the home page grows into the note page's player, which shows the same still until YouTube's player covers it. Browsers without view transitions just swap pages.
- Flashcards: Next slides the card aside and the next one lifts off the deck; Previous runs it backwards. Opening Revise drops the pile onto the desk, bottom card first.
- Avoid: mascots, leaderboards, sounds, looping animations, pictures beside the notes, pop-ups.
