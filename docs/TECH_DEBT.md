# Tech debt

Known problems we chose to leave for now. Remove an item once it's fixed.

All of these were found on 2026-10-31, on one 68-minute Biology lecture. Details: [M02 summary](milestones/M02-neet-study/SUMMARY.md).

## Exam marks never pass

- Problem: the teacher says "PYQ" 6 times in the test lecture, but every mark was dropped. Gemini translates the quote into English and quotes the fact, not the words calling it a PYQ.
- Likely fix: except `exam_quote` from the "everything in English" rule and show an example. If copied quotes still fail, compare the Hindi letters: the transcript stores `ज़` as one character, and Gemini may write it as two.
- Where: the notes prompt in `server/internal/pipeline/llm/note.go`, and `checkExamMark` in `check.go`.

## Teacher's mistakes reach students

- Problem: the teacher said Hooke used a simple microscope (it was a compound one), and that went into a bullet, a card and a quiz explanation. Students will read this as checked fact.
- Likely fix: unclear. We have no NCERT text to compare against.

## Sections can't go back

- Problem: sections must be in time order, so when a teacher returns to a topic the live highlight shows the wrong section. In the test lecture this happens around 50:00.
- Likely fix: let a section have more than one time range.

## Prompt rules Go can't check

- Problem: the section count is only in the prompt, so a thin note would still pass. A "Study Strategy" section got through, and 2 of 6 quiz explanations skip the wrong option.
- Likely fix: a Go check for the section count. The others need better prompts.

## Very short lectures

- Problem: a very short lecture may not hold the minimum 10 flashcards, and that step would then fail. A 6-minute video passed; shorter ones are untested.
- Likely fix: lower the minimum for short lectures.

## F key after clicking the video

- Problem: after a click inside the YouTube player, F goes to YouTube's fullscreen, because the page can't see key presses inside the player.
- Likely fix: none needed for now. The Focus button always works.

## Long queues fail jobs

- Found: M04.
- Problem: a job with no worker for about 30 minutes ends as failed, so if the queue is ever longer than that, waiting jobs fail too. Try again works.
- Likely fix: only time out jobs that have started.

## Real rate-limit errors are untested

- Found: M04.
- Problem: Google hasn't sent a real 429 yet, so the daily and per-minute handling is tested on a sample body written from memory.
- Likely fix: check the logs when the first real one happens.

## Permanent failures offer Try again

- Found: M04.
- Problem: a video with no captions, or one that's too long, still shows Try again. It fails the same way, but costs no AI calls.
- Likely fix: send a flag from the API saying a retry can't help.

## Study screens: real player untested

- Found: M06. Scoring, card keys and Watch from Revise now have Playwright tests (M08).
- Problem: nobody has used the screens with the real YouTube player, a screen reader, or measured contrast.
- Likely fix: one sitting with a finished note in the browser.

## No email check or password reset

- Found: M07.
- Problem: there's no email service, so emails aren't confirmed and a forgotten password can't be reset.
- Likely fix: an email provider, then a confirm link and a reset link.

## Signup isn't throttled, login throttling is in memory

- Found: M07.
- Problem: accounts can be made in bulk, so the total cap is the real limit on the AI bill. Login throttling forgets everything on a restart and only works with one API process.
- Likely fix: pick a `TOTAL_DAILY_CAP` before going public; throttle signup per address; keep throttle counts in Postgres if there's ever more than one API process.

## General template is untested on the real API

- Found: M07.
- Problem: the General prompts have only met a fake Gemini. Their notes, cards and quiz quality are unknown.
- Likely fix: one real run on a non-NEET lecture, then read the result.

## Smaller limits from M07 and M08

- The sample lecture must be loaded again after a NEET prompt change.
- The library reads titles out of each large saved transcript, and Revise sends every card with no paging.
- `/notes/<id>` opens the newest-template note, so an older-template library row opens the newer note.
- The daily count on the home page is from page load and doesn't change at midnight.

## Motion and dark mode, 2026-11-08

- Dark mode's contrast was checked by eye and by hand-worked numbers on the main colours, not measured on every screen.
- The still-to-player growth only plays going into a note: going back, the library loads its list after the page appears, so there's nothing to grow back into.
- Lecture stills load from YouTube's image server (`i.ytimg.com`), so YouTube sees which stills a student's library shows. The player already talks to YouTube, so this adds little.
