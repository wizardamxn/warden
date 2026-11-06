# M06 summary

Built on 2026-11-06: the flashcard and answer sheet screens in `apps/web`. Not tested: the user asked for no testing in this milestone. Only lint, type-check and the production build were run, and they pass.

## What was built

- Once the note exists, a quiet nav switches the right column between Notes, Flashcards and Answer sheet. The video stays on the left and is never rebuilt; all three views stay mounted and are only hidden.
- The end of the notes leads into the flashcards, and the last card leads into the answer sheet.
- Flashcards: a paper index card that turns over on click, Space or Enter. Arrow keys and buttons move between cards, with a "3 of 11" count. Keys are ignored while typing.
- Answer sheet: an OMR sheet with timing marks and bubbles numbered 1 to 4, filled in blue pen. Submit once: +4 right, −1 wrong, 0 blank. Red pen for wrong picks and margin marks, the highlighter on right answers, always with words too. Then the explanation and a Watch link. "Try the sheet again" resets it. Nothing is saved.
- Statement and assertion-reason questions keep their lines. Match questions show List I and List II side by side, or plain lines if the lists don't pair up.
- One celebration per submit: the light moves to the score and the highlighter sweeps under it. Off under reduced motion.
- "Watch 05:40" seeks the same player and shows Notes at that moment. Blocked videos open youtube.com instead.
- Flashcards or quiz still being made: their view says so from the real step status. Failed: which part couldn't be made, with Try again.
- Quiz options on direct and match questions are now shuffled in Go after the checks pass, because Gemini tended to put the right answer first. Statement and assertion-reason options keep NEET's fixed order.

## Choices made while building

- Bubbles are numbered 1 to 4 like NEET, not A to D, which would clash with match pairings like "A-II".
- A "Clear" link turns an answer back into a blank, because −1 marking makes blanking a real choice. Clearing moves keyboard focus to that question's first bubble.
- Answer highlights don't sweep: motion is only transform and opacity, so the sweep is kept for the score.

## Still weak

- A failed view shows its own Try again as well as the one under the video.

## Not tested

- Everything in a browser: the card turning, keys, scoring, marks and words, Watch seeking, Try the sheet again, the waiting and failed views, plain text, narrow width.
- Playwright: no new tests. Existing tests weren't run; none are expected to break.
- The real YouTube player, real API data, screenshots and the slop test on real pixels.
- Screen readers, and contrast checked by measurement.
