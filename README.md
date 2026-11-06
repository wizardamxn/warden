# Warden

Paste a NEET lecture link, get notes beside the video, then test yourself.

Full plan, MVP scope and build order: [docs/PLAN.md](docs/PLAN.md).

## Decided so far

- Name: Warden.
- Users: NEET students. The engine stays exam-agnostic; templates and quiz style are NEET.
- Source: YouTube links only.
- Transcript: YouTube captions for now. Self-hosted Whisper later, if captions aren't good enough.
- Languages: English-medium students first. Lectures can be spoken in English, Hindi or a mix of both, as long as they teach the English terms. The website, notes, flashcards and quizzes are always in English.
- Notes: short revision bullets, with the NCERT chapter name only (no page or line numbers).
- Study material: notes, flashcards and quiz are generated separately. Card and question counts scale with lecture length. Quizzes use the NEET question mix.
- Longest lecture: about 4 hours.
- Surface: desktop web app first. No Android app or browser extension.
- Free plan: 3 notes per user per day. Every lecture counts, even when the note is reused. Retrying a failed step doesn't use a daily note.
- AI calls: from Go, straight to the providers' HTTP APIs. No SDK.
- Website: Next.js on Netlify or Vercel.
- API and worker: Go, two separate programs, both running on an Android phone (Termux).
- Public access to the API: Cloudflare Tunnel on api.<domain>.
- Data: Supabase, used only as the Postgres database. Auth is our own.

## v1 features

1. Synced notes: paste a YouTube link, get notes (title, NCERT chapter name, TL;DR, key points, sections with timestamps, mnemonics, glossary) that follow the embedded video.
2. Test yourself: flashcards and a short quiz from each note, each linking back to its moment in the video.
3. Revise tab: cards from lectures older than 2 days. No algorithm, just a date filter.
4. Login (our own auth).
5. Daily limits: 3 notes per user per day, plus a total cap (number not decided yet).
6. Library: a simple list of your past notes.

## Website look and feel (v1)

- Look: light and optimistic, "light at the end of the tunnel". Warm paper background (not pure white), warm ink text (not pure black), a soft sunny glow where your attention should be, and a yellow highlighter for the live highlight and correct answers. Full rules: [docs/DESIGN.md](docs/DESIGN.md).
- Light only in v1. All colours live in one set of named theme colours, so dark mode can be added later by adding a second set.
- Layout: video on the left and notes on the right. Later, on phones: the video sticks to the top and the notes scroll under it.
- Video: YouTube's own embedded player, privacy-enhanced version (`youtube-nocookie.com`), controlled from our page (click a timestamp to jump).
- If a video can't be embedded: still make the notes. Show them on their own, and make each timestamp open the video on youtube.com at that second.
- Character: a NEET student's desk. Yellow highlighter, blue pen, red pen, and a quiz that works like an OMR sheet with +4 and −1 marking.
- First visit: a sample lecture anyone can try before signing up.
- Touches:
  - Live section highlight: a highlighter stroke sweeps under the heading of the section being played, and scrolls it into view.
  - Flashcard flip (a paper index card), and the highlighter marks a correct quiz answer.
  - Focus mode: one key narrows the light. The edges of the page fall into soft shade; only the video and the playing section stay lit.
  - A small celebration once, at the end of a quiz: the highlighter sweeps under the score.
- Motion: short (about 0.15–0.4 s) and only to show a change. Turned off when the device asks for less motion.
- Avoid: mascots, leaderboards, sounds, looping animations, pictures beside the notes, pop-ups.

## Later (v2 or after)

- Dark mode.
- Streaks or a "days studied" count. None in v1.
- Screenshots in notes, playlists, channel digests (these cost money).
- Spaced-repetition algorithm, other exams (JEE, UPSC, college), B2B or platform partnerships, a cheap Pro plan.
- Hindi-medium students and lectures.
- Phone layout for the website.
- Export to Markdown and PDF.

## Still open

- Total daily cap on notes across all users: the number.
- Netlify or Vercel (parked until deploy, see [DEPLOYMENT.md](DEPLOYMENT.md)).
- Whether we need Whisper, and notes layout on phones: see [docs/PLAN.md](docs/PLAN.md).

How the website must look, and why: [docs/DESIGN.md](docs/DESIGN.md).

## Folders

```
apps/web/          Next.js website
  src/app/         pages
  src/components/  one folder per feature (auth, notes, library, study) + shared ui
  src/lib/         api client, website session
  e2e/             Playwright tests
server/            Go project: API + worker
  cmd/api/         API program
  cmd/worker/      worker program
  cmd/migrate/     applies database changes
  internal/        one folder per feature (auth, users, notes, study, jobs, limits)
                   + core, shared, and pipeline (captions, transcribe, llm)
  migrations/      Postgres database changes, Docker locally and Supabase later
  tests/           integration and end-to-end tests
evals/             note-quality checks
deploy/            phone and website hosting setup
docs/              architecture, milestones
scripts/           dev helpers
.github/workflows/ CI, later
```


__AMANS THOUGHTS__ 

SOMETHING LIKE WE CAN GRAB ALL THE PAST QUESTIONS FROM NEET AND THEN WHEN WE WATCH THOSE LECTURES OR PASTE THOSE NOTES THE AI GRABS THE RELEVANT QUESTIONS AND ADDS THEM IN THE QUIZ/TESTS/ETC..? 

HMM LETS THINK LETS THINK. 

AN AGENT!? HMM WILL THINK LATER.