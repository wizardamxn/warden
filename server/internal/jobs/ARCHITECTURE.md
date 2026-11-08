# Jobs

The job list: the API adds a job, the worker claims it and runs its steps. Two tables, `jobs` (one row per video and template) and `job_steps` (one row per step). Each student's `library` lives here too, because it points at jobs.

## Save on every step

A job runs fixed steps: captions, notes, flashcards, quiz. There is no agent loop.

- Each step is saved on its own, with a marker in the database saying it's done.
- The result and its marker are written together in one update, so we never have a "done" marker with no data, or data with no marker. The database also refuses "done" with no result.
- A retry only runs steps with no "done" marker. The AI never regenerates something already saved.
- Notes, flashcards and quiz don't depend on each other, so if one fails the others still finish and stay saved.
- The transcript is saved too, so a retry doesn't download it again, and all three AI steps read the same one.
- If captions fail, the other three fail with the same sentence, because they have nothing to read. Try again runs all four.
- If the AI's answer fails the checks, the worker sends back the list of errors once. If it still fails, that step stops as failed.
- The website shows what's ready, plus a "Try again" button. It runs only the failed steps and doesn't use a daily note.
- Tokens and cost add up over all attempts of a step, so they show what was really spent.

## Claiming

- A worker claims a job with `FOR UPDATE SKIP LOCKED`, so two workers can't take the same one. It then holds the job for 10 minutes and renews the hold while it runs, so a slow step isn't mistaken for a stuck one.
- Each step's attempt number proves it still belongs to its worker. A worker that lost a step can't save over it.
- On Ctrl+C the worker stops the step, puts it back to free and lets go of the job, so the next worker starts at once. A result that had already arrived is still saved.

## When a worker dies

The website polls until a job ends, so a job must always end, even with no worker left. These rules run in the worker's claim, and in the API's read of the job, so polling alone is enough:

- About 10 minutes after the last sign of life, a step stuck as "running" goes back to free and a worker retries it.
- A step that has had 3 attempts stops as failed instead, so one that keeps killing workers can't spend AI calls forever.
- A job nobody picks up for about 30 minutes ends: its unfinished steps fail with a plain sentence. This covers a dead worker, a clean stop with no restart, and a job no worker ever claimed. Try again gives it a new start.

shortcut: with a queue over 30 minutes long, waiting jobs also end this way, even with a healthy worker. Compare against a worker heartbeat if the queue ever gets that long.

## Job status

Worked out from the steps every time, never stored:

- `running`: a step is running, or a worker holds the job.
- `queued`: steps are waiting and no worker holds the job.
- `done`: every step is done.
- `partial`: notes exist, but flashcards or quiz failed.
- `failed`: no note exists.

## Reusing saved results

Results are saved by YouTube video ID, template and prompt version. If a second student pastes the same lecture under the same template, they get the saved note with no AI call. The same lecture under the other template is a second job, made with that template's prompts. A new prompt version generates it fresh, and only for the steps whose prompt changed, the next time someone requests that lecture.

The worker reads each job's template from the database and runs the pipeline with it.

## Library and Revise

- A student's library is a row per lecture and template, with `added_at`. Adding the same lecture again keeps the first `added_at`.
- Adding a lecture checks the [limits](../limits/ARCHITECTURE.md) and makes the job in the same transaction.
- The library list shows each lecture's title, channel and length from its saved captions (empty until then) and the job's status, newest first.
- Revise sends the flashcards of every lecture added more than 2 days ago, oldest lecture first. No algorithm, just the date.
- Every query filters by the student's own user ID.

shortcut: the library reads titles out of each saved transcript, which is large. Copy title, channel and length onto `jobs` if the library gets slow.

## The sample lecture

`SaveFinished` stores a whole note made elsewhere as a finished NEET job. `cmd/sample` uses it.

shortcut: a note has no transcript, so the sample's captions step is saved with no lines and a new prompt version can't remake it. Load the sample again after a prompt change.
