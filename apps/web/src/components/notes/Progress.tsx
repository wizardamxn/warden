import { isFinished, type NoteJob, type Step } from "@/lib/api/notes";

export type ProgressProps = {
  job: NoteJob | null;
  problem: string;
  retrying: boolean;
  onTryAgain(): void;
};

const DOING: Record<Step["name"], string> = {
  captions: "Getting the captions.",
  notes: "Writing the notes.",
  flashcards: "Making the flashcards.",
  quiz: "Making the quiz.",
};

// Read only from the steps. In the moment between two steps, this names the next one.
function doing(job: NoteJob): string {
  if (job.status === "queued") return "In the queue.";
  const step = job.steps.find(({ status }) => status === "running") ?? job.steps.find(({ status }) => status === "pending");
  return step ? DOING[step.name] : "Finishing up.";
}

// Stands where the note will start, until the note exists.
export function Waiting({ job, problem, retrying, onTryAgain }: ProgressProps) {
  const ended = job !== null && isFinished(job);
  const reason = job?.error || job?.steps.find(({ status }) => status === "failed")?.error;

  return (
    <div className="waiting" aria-live="polite">
      {problem ? (
        <h1>{problem}</h1>
      ) : ended ? (
        <>
          <h1>The notes couldn&apos;t be made.</h1>
          {reason && <p className="problem">{reason}</p>}
        </>
      ) : job ? (
        <>
          <h1>{doing(job)}</h1>
          <p className="sub">This can take several minutes. Watch the lecture meanwhile; the notes appear here as soon as they&apos;re written.</p>
        </>
      ) : (
        <p className="sub">Opening the note.</p>
      )}
      {(problem || ended) && (
        <button className="primary" type="button" disabled={retrying} onClick={onTryAgain}>
          Try again
        </button>
      )}
    </div>
  );
}

// Fills the Flashcards or Answer sheet view while that part isn't there, read only from its own step.
export function NotReady({ part, job, retrying, onTryAgain }: { part: "flashcards" | "quiz" } & ProgressProps) {
  const step = job?.steps.find(({ name }) => name === part);
  const status = job?.status === "queued" ? "pending" : step?.status;

  return (
    <div className="not-ready" aria-live="polite">
      {status === "failed" ? (
        <>
          <h2>The {part} couldn&apos;t be made.</h2>
          {step?.error && <p className="problem">{step.error}</p>}
          <button className="primary" type="button" disabled={retrying} onClick={onTryAgain}>
            Try again
          </button>
        </>
      ) : status === "running" || status === "pending" ? (
        <>
          <h2>{status === "running" ? DOING[part] : `Waiting to make the ${part}.`}</h2>
          <p className="sub">This part appears here as soon as it&apos;s made. Read the notes meanwhile.</p>
        </>
      ) : (
        <h2>This lecture has no {part}.</h2>
      )}
    </div>
  );
}

// One quiet line under the lecture, once the note is on screen. Empty when there's nothing to say.
export function StatusLine({ job, problem, retrying, onTryAgain }: ProgressProps) {
  const lost = job?.status === "partial" ? job.steps.filter(({ status }) => status === "failed").map(({ name }) => name) : [];
  const line = problem || (lost.length > 0 ? `The ${lost.join(" and the ")} couldn't be made.` : job && !isFinished(job) ? doing(job) : "");

  return (
    <p className="sub" aria-live="polite">
      {line}
      {(problem || lost.length > 0) && (
        <>
          {" "}
          <button className="quiet again" type="button" disabled={retrying} onClick={onTryAgain}>
            Try again
          </button>
        </>
      )}
    </p>
  );
}
