# Warden — Starting Plan

Warden turns a YouTube lecture into study material, so a student learns from the video instead of only watching it.

_Last updated: 2026-10-31_

---

## Decisions (locked)

| Area | Decision | Why |
|---|---|---|
| Cohort | **Anyone studying from YouTube lectures** | A universal study helper, not tied to one exam. Changed 2026-11-08: it started as NEET-only (largest lecture-watching cohort), and templates (NEET first, plus General) handle the difference between exams. |
| Source | **YouTube links only** | No paid-course capture, no extension. Legally boring. |
| Transcript | **YouTube captions for now** | No Whisper headache to start. If captions turn out not to be good enough, add self-hosted Whisper (small/medium). |
| Surface | **Desktop web app first** | Paste a link → embedded player with notes beside it, built for laptop screens. Phone layout comes later. No app store, no extension. |
| Look | **Warm paper + yellow highlighter** | No mascots, streaks, leaderboards, sounds, pop-ups. |
| Pricing | **Free, 3 notes/day** | Monetisation deferred. Direction: cheap Pro later. |
| Infra | **Phone server, about ₹2 to ₹4 per lecture at paid prices** | Measured in step 2 on one lecture, covering notes, flashcards and quiz. Fine until it isn't. Don't touch until it breaks. |

---

## MVP scope

1. **Synced notes** — link → transcript → notes that follow the player. Highlight moves with playback.
2. **Test yourself** — notes end → flashcards + short quiz. Every card/question links back: "Watch 05:40".
3. **Revise tab** — shows cards from lectures older than 2 days. No algorithm, just a date filter. This is what makes it a habit instead of a one-time tool.
4. **Accounts + 3/day limit.**

## Out of scope until MVP is in students' hands

- Spaced-repetition algorithm
- Other exams (JEE, UPSC, college)
- Android app / browser extension
- B2B / platform partnerships
- Any marketing
- Any scaling work

---

## Build order

1. **Pipeline as a script.** YouTube link → captions → notes JSON with timestamps.
   Test on one real NEET lecture, then by hand.
   _If notes are bad here, nothing else matters._
2. **NEET prompt templates.** Done on one lecture. Per lecture: notes with the NCERT chapter name + mnemonics, then flashcards and MCQs. Notes, flashcards and quiz are generated separately. Card and question counts scale with lecture length. MCQs use the NEET mix: direct, statement-based, assertion-reason, match-the-column.
3. **Web app.** Done locally. Player + synced notes.
4. **Flashcards + quiz screens.** Done, tested with a mocked API.
5. **Revise tab.** Done, with the library.
6. **Accounts + daily limit.** Done. The student picks a template when they sign up. They can switch it in settings later, for new notes only.
7. **Templates.** Built. General's prompts haven't met the real Gemini API yet. NEET (today's prompts) and General. The worker reads the student's template from the database, and finished notes are reused per video and template.

---

## Done means

10 students use it on their own lectures for a week, and at least a few come back to the Revise tab unprompted.

That's the signal the learning loop works. Then — and only then — marketing.

---

## Open questions (not blocking)

- Are captions good enough, or do we need Whisper? Answer in step 1. If Whisper: model size vs speed on the phone server.
- Later, for the phone layout: notes below the video, or a swipeable panel?
