# M02 summary

One real lecture tested, on 2026-10-31, on the free tier: the same 68-minute PW NEET English Biology class as M01 (`k7Zb1iOV-ys`). More testing is manual.

## Results on this lecture

- Sections: 10, was 3. Bullets: 40, was 13. Viruses now has its own section.
- Flashcards: 11. Quiz questions: 6 (2 direct, 2 statement, 1 assertion-reason, 1 match).
- NCERT chapter: "Cell: The Unit of Life".
- Mnemonics: 1, the teacher's own.
- Exam marks: 0. See "Still weak".

## What worked

- A full `note.json` from one worker run. A re-run with every step saved makes no AI calls.
- Changing only the quiz prompt re-ran only the quiz. Notes and flashcards were reused.
- The fix-up loop fixed real problems: wrong section times, and quiz questions written on one line.
- A failed flashcards step still writes the note, with `failed_steps` filled in and a non-zero exit. A failed notes step stops with the error and writes nothing. Both were run on the real program, with the network blocked.

## What went wrong and was fixed

- Asking in words for more sections changed nothing: still 3. Putting a number in the prompt ("Write about 10 sections") gave 10. The number is about 1 section per 7 minutes, min 3, max 30. Go doesn't check it.
- The quiz had a wrong answer key: it called "karyon means nucleus" the explanation for prokaryotes lacking a nucleus. The prompt now says R must be the actual cause of A.
- The quiz had a question whose answer didn't answer it, and a match question whose answer was simply A-I, B-II, C-III, D-IV. The prompt now rules out both.

## Spot-check

Every timestamp below was read against the transcript, and every question was read for correct biology.

- Flashcards: all 11 land on the fact.
- Quiz: all 6 land on the topic. All 6 answers are right.
- Sections: 8 of 10 land on the moment. "Levels of Biological Organization" starts about 80 seconds late, and "Fundamental Unit of Life" about 20 seconds late.
- No card or question tests something the lecture doesn't teach.

## Still weak

- No exam marks. The teacher says "previous year question" or "PYQ" 6 times, but every mark was dropped. Gemini translates the quote into English and quotes the fact, not the words calling it a PYQ. The likely fix is to except `exam_quote` from the "everything in English" rule and show an example. Not tested, the budget ran out.
- M01 says this lecture named no exam years. That's true, but it does name PYQs, so marks are possible here.
- Teacher's slip repeated: a bullet, one card and one quiz explanation say Hooke used a simple microscope. It was a compound one. The prompt's "leave out what looks wrong" didn't catch it.
- "Father of cytology" is Robert Hooke in one card. That's what the teacher says, but sources differ.
- A "Study Strategy" section got through, though the prompt says to leave out study advice. Asking for 10 sections may have pushed it in.
- Missing from the note: how life began (chemical then biological evolution), and how better microscopes helped.
- A few small additions the teacher never said, all true: prokaryotes lack membrane-bound organelles, "karyon" is Greek, Hooke used thin slices. The title "Viruses and Cell Theory" also names cell theory, which wasn't taught.
- Sections can't go back. The teacher returns to Hooke at about 50:00 after "Basic Definitions", so that part of the video will highlight the wrong section.
- Flashcards skip things the teacher stressed: cytoplasm as the main arena (a PYQ), the fathers of biology and botany, the hierarchy.
- 2 of 6 explanations don't say why a wrong option is wrong. Go can't check this.
- Correct answer positions lean to the first option: 3 of 6.

## Cost

- This lecture, final version: ₹3.68 at paid prices (notes ₹1.61, flashcards ₹0.63, quiz ₹1.44). Notes and quiz each needed a fix-up.
- With no fix-ups: about ₹2.10.
- Testing used 10 answered calls, ₹7.15, plus 2 calls refused with a 503, which aren't billed. No rate limit was hit.

## Not tested

- A Hindi-spoken or Hinglish lecture, Physics, Chemistry, a long lecture.
- An exam mark that passes, or a mark with a real year.
- The minimum counts on a short lecture. A very short one may not have 10 cards in it, and that step would then fail.
