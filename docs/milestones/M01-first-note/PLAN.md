# M01: First note

Status: done on one real lecture. The user chose one lecture instead of 5 (free tier) and tests further by hand. Results: [SUMMARY.md](SUMMARY.md).

This is step 1 of the build order in [docs/PLAN.md](../../PLAN.md). Flashcards, the quiz, NCERT references and mnemonics are step 2.

## Goal

Paste a YouTube link to a NEET lecture in a terminal and get a finished note back as a file: title, TL;DR, key points, sections with timestamps, glossary. No website, no login, no database yet.

## Why first

- It's the product. If the notes aren't good, nothing else matters.
- It answers the open questions: how usable YouTube's captions are on NEET lectures, how good Gemini's notes are, and the real cost per note.
- It locks the note's shape, which the note page, study screens and Revise tab all read.

## In this milestone

- Run the real worker program once, by hand, on one link: `go run ./cmd/worker -url <link> -out note.json`. Later milestones make the same worker take jobs from Supabase instead.
- Captions: yt-dlp downloads the lecture's captions only (no video, no audio), in the language it was taught in. Prefer manual captions, then YouTube's auto captions. The one NEET lecture sampled so far only had Hindi auto captions, in Devanagari.
- Clean the captions into a transcript with a start time on every line, removing any repeated or empty lines.
- Notes: one call to Gemini 3.1 Flash-Lite over its plain HTTP API (no SDK), asking for JSON in a fixed shape.
- Check the AI's answer before saving (it's untrusted input): required fields present, timestamps inside the video's length and in order. If it fails, send Gemini the list of errors and ask it to fix them, one time only, then stop with a clear error.
- Save each finished step as a local file (transcript, notes), so a re-run skips it. This is the same idea as the database markers later.
- Log tokens used and the cost of each note.

## Not in this milestone

- Flashcards, quiz, NCERT references, mnemonics (step 2).
- Jobs table, API, Supabase, login, limits, website.
- Running on the phone.
- Whisper. Only if captions turn out not to be good enough. Videos with no captions stop with a clear "no captions" error.

## How it works

1. Read the link and pull out the video ID. Reject anything that isn't a YouTube video link.
2. yt-dlp: get the title, channel and length. Reject lectures over the length limit.
3. yt-dlp: get the captions as YouTube's `json3` format (each line has an exact start time).
4. Build the transcript: `[655] <line>`, in plain seconds. Given `[10:55]`, Gemini wrote 1055 instead of 655.
5. Send the transcript and instructions to Gemini. Ask for JSON matching the note shape below.
6. Check the JSON, then write `note.json`.

## Note shape (draft)

```
title, channel, duration_seconds, source ("captions" or "auto-captions")
tldr
key_points: [text]
sections: [{ start_seconds, title, bullets: [text], exam_mark, exam_quote }]
glossary: [{ term, definition }]
```

`exam_mark` is what the teacher said about NEET at that point ("Asked 2018, 2024"), and stays empty if they said nothing. `exam_quote` is their exact words. Go checks the quote is in the transcript and that every year in the mark is in the quote, so an invented mark can't get through.

Step 2 adds flashcards and quiz questions, each with a `start_seconds`. That's what makes "Watch 05:40" work.

## Where the code goes

- `server/cmd/worker/`: the worker program, with the one-off `-url` mode.
- `server/internal/pipeline/captions/`: run yt-dlp, read `json3`, clean the transcript.
- `server/internal/pipeline/llm/`: the prompt, the Gemini HTTP call, checking the answer.
- `server/internal/pipeline/`: the steps in order (captions, then notes).
- `server/internal/notes/`: the note shape, so everything else uses the same one.
- `server/internal/core/`: settings. The Gemini key comes from `server/.env` (git ignores it). A real environment variable with the same name wins over the file.

## How we build

Claude writes all the code. The user dropped the plan to write pieces and learn Go for now.

## Tests

- Caption cleaning: a saved `json3` file gives the expected transcript, with no repeated lines.
- Answer checking: good JSON passes; out-of-range timestamps or a missing field fail.
- No test calls the real Gemini API. Real notes are checked by reading them.

## Done when

- One real NEET lecture gives a valid `note.json`.
- Spot-check the notes: are NEET terms right even where the captions misheard them, and do the timestamps match what's said at that moment?
- Cost per note is logged and compared with the ₹0.2 we expected.
- A link that isn't YouTube, a lecture over the limit, and a video with no captions each give a clear error.
- Write `SUMMARY.md`: what worked, how good the captions were, real costs, prompt notes, and whether we need Whisper.

## Setup needed

- Go 1.27.0: installed (winget).
- yt-dlp 2026.10.21: installed (winget). It also brought Deno, which yt-dlp needs to read YouTube pages, and its own ffmpeg build.
- ffmpeg: installed (not needed unless we add Whisper).
- A Gemini API key from Google AI Studio, as `GEMINI_API_KEY=...` in `server/.env`. Never committed. Not set yet.

## Decided

- **Transcript:** YouTube's captions, not Whisper, for now. Less work to start, and M01 shows whether they're good enough.
- **Languages:** English-medium students first. Lectures can be spoken in English, Hindi or a mix of both, as long as they teach the English terms. Notes are always in English, with English terms ("sperm", never "shukranu"). Hindi-medium comes later.
- **Note style:** short revision bullets, not paragraphs.
- **Longest lecture allowed:** about 4 hours. 2 of 6 sample lectures from a search ran 3:52.
- **Testing:** one real lecture on the free tier. More testing is manual.

## Risks

- YouTube can treat a lecture spoken in English as Hindi, and then its only real captions are English sounds written in Hindi script ("हैविंग द प्रोटोप्लाज्म"). Seen on a PW NEET English lecture. The AI has to read these back into English. Check this first in the test lectures, because it decides whether we need Whisper.
- Auto captions have no punctuation and mishear scientific terms ("myasthenia gravis" came out as "माथी निया ग्रेविस" in a sample lecture). The AI has to correct them.
- Captions only hold what's said. Anything written on the board is lost, which may hurt Physics and Chemistry most. Not checked yet.
- YouTube can rate-limit caption downloads (we hit an HTTP 429 on one sample).
- YouTube changes often break yt-dlp. Keep it updated; it's a known moving part.
- Since 18 Sep 2026, Google limits Gemini 2.5 Flash-Lite to past users, so we use `gemini-3.1-flash-lite`: $0.25 per million tokens in and $1.50 out on Google's pricing page (Oct 2026), and its earliest shutdown is listed as 7 May 2027. Google recommends `gemini-3.5-flash-lite` ($0.30 and $2.50) for new projects. The model is one line in `llm/gemini.go`.
- Long lectures mean long transcripts, so a note will likely cost well over the ₹0.2 we expected, which was based on the old model's prices.
- A 4-hour lecture may hit Gemini's output limit in one call. M01 measures this, and we split it into chunks only if it does.
