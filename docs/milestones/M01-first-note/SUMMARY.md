# M01 summary

One real lecture tested, on 2026-10-31, on the free tier: a 68-minute PW NEET English Biology class (`k7Zb1iOV-ys`). More testing is manual.

## What worked

- Link to `note.json` works end to end, and a re-run skips finished steps.
- Gemini read English written in Hindi script back into correct English, including names like Robert Hooke and Dujardin. For this lecture, captions were good enough without Whisper.
- All 3 section timestamps landed on the moment the topic starts.

## What went wrong and was fixed

- Given `[10:55]`, Gemini wrote 1055 instead of 655. The transcript now uses plain seconds, and Go checks every section starts on a real transcript line.
- Gemini invented exam marks ("Asked 2018, 2024") that this lecture never said. The checks caught them. A mark that still fails after the fix-up attempt is dropped and the note is kept.
- Google returned 503 "high demand" often. The worker now waits and retries, up to 5 attempts.

## Still weak

- The note is thin: 3 sections and 13 bullets for 68 minutes. Viruses, for one, didn't get a section.
- No exam mark has passed yet. This lecture named no exam years, so it doesn't show whether real marks get through.

## Cost

- One attempt: about 26,000 input and 850 output tokens, about ₹0.7 at paid prices.
- This run took two attempts: ₹1.35. Paid prices are in `llm/gemini.go`; the rupee figure uses a fixed rate of 85.
- The ₹0.2 per note estimate is out of date.

## Not tested

- A Hindi-spoken or Hinglish lecture, Physics, Chemistry, a long lecture, or a video with no captions.
