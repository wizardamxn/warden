# Pipeline

What the worker does for one job, step by step: captions (speech to text later, if needed), then notes, flashcards and quiz. Those three each read the whole transcript, are saved on their own, and don't depend on each other.

- `Run` is the one pipeline. It saves through a `Store`, so the worker's jobs (Postgres) and a one-off `-url` run (files in `.work/<video id>/`) do the same work.
- A store says whether each step should run, and keeps its result. A step that's saved doesn't run.
- `PlainError` turns an error into the sentence students read. Raw errors can hold Gemini's JSON, so they stay in the logs.
