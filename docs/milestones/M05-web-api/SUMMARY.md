# M05 summary

Built on 2026-10-31: the website now talks only to the Go API. Tested in real Chrome against a stand-in API, because the Go API wasn't running yet. A later real check passed: a pasted link opened its note from the real API, with no console errors.

## What worked

- Pasting a link sends it to the API and opens `/notes/<video id>` straight away.
- The page checks the API every 2 seconds until the status is `done`, `partial` or `failed`, and stops on leaving.
- The words come only from real step statuses. The glow grows with each finished step.
- The video stays on screen while waiting. The note appears as soon as it exists, while flashcards and quiz carry on.
- The real YouTube video kept playing through the note arriving: its clock ran from 7.9 s to 14.1 s and never jumped back.
- Failed note: the plain error and Try again. Partial: the note stays usable, plus one quiet line and its Try again.
- Server down: a clear message and Try again, with the note and player left in place.
- One lost check is forgiven. Three in a row show the message.
- Tested with Playwright with the API mocked. Breaking polling on purpose made the tests fail, as it should.

## What went wrong and was fixed

- At 1920 px the glow sat left of the video, because it was fixed to the page while the content is centred. It now follows the column, which moved it a little on the home page too.
- The glow started so small it hid behind the player. It now starts at 0.6 and adds 0.1 per finished step.
- The down message said "Try again" twice. It's now "Warden's server isn't answering. Give it a minute."

## Still weak

- "Making the flashcards." shows before those screens exist. It stays so a Try again click gets a visible response.
- A permanent failure, such as no captions, still offers Try again.

## Not tested

- The Try again button in the browser. A real retry was tested directly on the API.
- Watching real progress in the browser while a new note is made.
- A blocked-embed video while still waiting.
