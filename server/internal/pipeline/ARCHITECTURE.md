# Pipeline

What the worker does for one job, step by step: captions (speech to text later, if needed), then notes, flashcards and quiz. Those three each read the whole transcript, are saved on their own, and don't depend on each other.

- `Run` is the one pipeline, given a template. It saves through a `Store`, so the worker's jobs (Postgres) and a one-off `-url` run (files in `.work/<video id>/`) do the same work.
- Each step's version comes from the template's prompt versions. A one-off run names its files `notes-<version>.json` and so on, so NEET's files keep their old names and General's (`notes-general-v1.json`) never mix with them. Both share `transcript.json`.
- `Assemble` sets the note's `template` from the job, never from the AI.
- A store says whether each step should run, and keeps its result. A step that's saved doesn't run.
- `PlainError` turns an error into the sentence students read. Raw errors can hold Gemini's JSON, so they stay in the logs.
