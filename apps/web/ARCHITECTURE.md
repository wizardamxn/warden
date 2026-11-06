# Website

The Next.js website, hosted on Netlify or Vercel. It talks only to the API at api.<domain>.

The API address is the `NEXT_PUBLIC_API_URL` setting, default `http://127.0.0.1:8080`. It's fixed when the website is built, so a deployed build must set it. All calls go through `src/lib/api/notes.ts`.

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
- Answer sheet: the whole quiz as an OMR sheet, scored like NEET (+4 right, −1 wrong, 0 blank). Each question is a native radio group. Submit once, then each question shows its mark, the right answer, the explanation and its Watch link. Nothing is saved.
- Right and wrong are always told in words as well as colour ("Right answer", "Your answer, wrong", "+4 right").
- Match questions show List I and List II side by side when the text holds both lists that pair up; otherwise the question shows as its own lines.
- While flashcards or quiz are still being made, their view says so from the real step status. A failed step says which part couldn't be made, with Try again.
- Quiz and flashcard text comes from an AI too, so it's plain text only.

## Theme

- Light only in v1. Colours, fonts, spacing, the glow and the highlighter: [docs/DESIGN.md](../../docs/DESIGN.md).
- Every colour has a name ("background", "text", "accent", ...) defined in one place. Screens use only the names, never raw colour values.
- Dark mode later = one more set of values for the same names, plus a toggle. No screen changes.

## Layout

- Desktop first: video on the left, notes on the right.
- Phone, later: video sticks to the top, notes scroll under it.
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
- The quiz's one celebration: after submitting, the light moves from the video to the score and the highlighter sweeps under it, once per submit.
- Avoid: mascots, leaderboards, sounds, looping animations, pictures beside the notes, pop-ups.
