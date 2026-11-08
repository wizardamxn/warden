# Sample program

Stores a finished note as the sample lecture: the note anyone can open at `GET /v1/sample` before signing up. It saves the note as a finished NEET job, so a student who pastes the same link also gets it at once, with no AI calls.

From `server/`, after `go run ./cmd/migrate`:

```
go run ./cmd/sample -video k7Zb1iOV-ys -note ../apps/web/fixtures/k7Zb1iOV-ys.json
```

Then add `SAMPLE_VIDEO_ID=k7Zb1iOV-ys` to `server/.env` and restart the API.

- `-note` is a `note.json` file, as the worker's `-url` run writes. Its `video_id` must match `-video`, and it needs sections, flashcards and a quiz.
- Running it again replaces the stored note.
- A note has no transcript, so after a NEET prompt version changes, load the sample again rather than letting it be remade.
