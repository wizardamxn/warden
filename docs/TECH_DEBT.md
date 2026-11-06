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

## Study screens are untested

- Found: M06.
- Problem: the flashcard and answer sheet screens were built without any testing, by choice. Nothing has been seen working in a browser.
- Likely fix: open a finished note and try every view, then add Playwright tests for scoring, keys and Watch.
