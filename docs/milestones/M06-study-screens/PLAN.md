# M06: Flashcards and answer sheet

Status: built, untested. The user asked for no testing in this milestone, so only lint, type-check and the production build were run.

## Goal

Step 4 of the build order. The end of the notes leads straight into testing yourself: flashcards, then a quiz marked like the real NEET paper. Every card and question links back to its moment in the video.

## In this milestone

- `Note` in `src/lib/api/notes.ts` gains `flashcards`, `quiz` and `failed_steps`, all optional for older notes.
- The video stays on the left. A quiet nav switches the right column between Notes, Flashcards and Answer sheet, without rebuilding the player.
- The end of the notes: "Reading it isn't the same as knowing it." and one primary button into the flashcards.
- Flashcards: a paper index card (`#FFFDF8`, faint blue rules, one red line, softly lifted) that turns over. One at a time, "3 of 10", click or Space or Enter to turn, arrow keys and buttons to move. Keys ignored while typing.
- Answer sheet: an OMR sheet with timing marks, bubbles numbered 1 to 4 like NEET, filled in blue pen. Submit once: +4 right, −1 wrong, 0 blank, red pen for wrong answers and margin marks, the highlighter on right answers, then the explanation and its Watch link. "Try the sheet again" resets it.
- Statement and assertion-reason questions keep their lines. Match questions show List I and List II side by side.
- One celebration, once per submit: the light moves to the score and the highlighter sweeps under it. Off under reduced motion.
- "Watch 05:40" seeks the same player and shows Notes at that moment. Blocked videos open youtube.com at that second.
- Flashcards or quiz still being made: their view says so from the real step status. Failed: which part couldn't be made, with Try again.
- All AI text stays plain text.

## Not in this milestone

- Revise, accounts, saving scores, the phone layout, any change in `server/`.
- Playwright tests and browser checks, by the user's choice.

## Done when

- Lint, type-check and the production build pass.
- Later, when testing is back: Playwright covers the cases listed in the summary, and real Chrome screenshots of the card front and back, the empty sheet, the marked sheet and narrow width pass the slop test.
