"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { getNote, isFinished, retryNote, type NoteJob } from "@/lib/api/notes";
import { NoteView } from "./NoteView";
import "./notes.css";

// shortcut: polling trusts the server to end every job, add a time limit here if a dead worker can leave one running forever.
const POLL_MS = 2000;
// One lost request is a blip, not an outage.
const GIVE_UP_AFTER = 3;

export function NoteScreen({ videoId }: { videoId: string }) {
  const [job, setJob] = useState<NoteJob | null>(null);
  const [missing, setMissing] = useState(false);
  const [problem, setProblem] = useState("");
  const [retrying, setRetrying] = useState(false);
  // Polling ends when the job finishes or the server is gone. Each new round starts it again.
  const [round, setRound] = useState(0);

  useEffect(() => {
    const abort = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    let misses = 0;

    async function poll() {
      try {
        const next = await getNote(videoId, abort.signal);
        if (!next) return setMissing(true);
        misses = 0;
        setJob(next);
        if (!isFinished(next)) timer = setTimeout(poll, POLL_MS);
      } catch (failure) {
        if (abort.signal.aborted) return;
        if (++misses < GIVE_UP_AFTER) timer = setTimeout(poll, POLL_MS);
        else setProblem((failure as Error).message);
      }
    }

    poll();
    return () => {
      abort.abort();
      clearTimeout(timer);
    };
  }, [videoId, round]);

  async function tryAgain() {
    setProblem("");
    // If the server couldn't be reached, just look again. Only a job that really failed is retried.
    if (job && !problem) {
      setRetrying(true);
      try {
        const next = await retryNote(videoId);
        if (!next) return setMissing(true);
        setJob(next);
      } catch (failure) {
        return setProblem((failure as Error).message);
      } finally {
        setRetrying(false);
      }
    }
    setRound((count) => count + 1);
  }

  if (!missing) return <NoteView videoId={videoId} job={job} problem={problem} retrying={retrying} onTryAgain={tryAgain} />;

  return (
    <div className="page">
      <div className="glow" aria-hidden="true" />
      <main className="wrap">
        <div className="nav">
          <Link className="brand" href="/">
            Warden
          </Link>
        </div>
        <div className="home">
          <h1>There&apos;s no note for this video yet.</h1>
          <Link className="link" href="/">
            Paste its link to make one
          </Link>
        </div>
      </main>
    </div>
  );
}
