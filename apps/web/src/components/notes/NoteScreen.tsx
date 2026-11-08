"use client";

import Link from "next/link";
import { useEffect, useState, type ReactNode } from "react";
import { needsLogin } from "@/lib/api/client";
import { getNote, getSample, isFinished, retryNote, type NoteJob } from "@/lib/api/notes";
import { useMe, useToLogin } from "@/lib/auth/session";
import { Nav } from "../ui/Nav";
import { NoteView } from "./NoteView";
import "./notes.css";

// shortcut: polling trusts the server to end every job, add a time limit here if a dead worker can leave one running forever.
const POLL_MS = 2000;
// One lost request is a blip, not an outage.
const GIVE_UP_AFTER = 3;

// A student's note, or the sample lecture when there's no `videoId`. Only the sample opens without a login.
export function NoteScreen({ videoId, start = 0 }: { videoId?: string; start?: number }) {
  const sample = !videoId;
  const toLogin = useToLogin();
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
        const next = videoId ? await getNote(videoId, abort.signal) : await getSample(abort.signal);
        if (!next) return setMissing(true);
        misses = 0;
        setJob(next);
        if (!isFinished(next)) timer = setTimeout(poll, POLL_MS);
      } catch (failure) {
        if (abort.signal.aborted) return;
        if (needsLogin(failure)) return toLogin();
        if (++misses < GIVE_UP_AFTER) timer = setTimeout(poll, POLL_MS);
        else setProblem((failure as Error).message);
      }
    }

    poll();
    return () => {
      abort.abort();
      clearTimeout(timer);
    };
  }, [videoId, round, toLogin]);

  async function tryAgain() {
    setProblem("");
    // If the server couldn't be reached, just look again. Only a job that really failed is retried, and never the sample.
    if (job && !problem && videoId) {
      setRetrying(true);
      try {
        const next = await retryNote(videoId);
        if (!next) return setMissing(true);
        setJob(next);
      } catch (failure) {
        if (needsLogin(failure)) return toLogin();
        return setProblem((failure as Error).message);
      } finally {
        setRetrying(false);
      }
    }
    setRound((count) => count + 1);
  }

  if (missing) {
    return (
      <Plain signedIn={!sample}>
        {sample ? (
          <h1>There&apos;s no sample lecture yet.</h1>
        ) : (
          <>
            <h1>There&apos;s no note for this video yet.</h1>
            <Link className="link" href="/">
              Paste its link to make one
            </Link>
          </>
        )}
      </Plain>
    );
  }
  if (sample) return <Sample job={job} problem={problem} retrying={retrying} onTryAgain={tryAgain} />;
  return <NoteView videoId={videoId} start={start} signedIn={job !== null} job={job} problem={problem} retrying={retrying} onTryAgain={tryAgain} />;
}

// The sample has no video ID until it arrives, and anyone can open it, so it asks who's here for the top line.
function Sample(progress: { job: NoteJob | null; problem: string; retrying: boolean; onTryAgain(): void }) {
  const { user } = useMe(false);
  const signedIn = user === undefined ? undefined : user !== null;
  const id = progress.job?.video_id;
  if (id) return <NoteView videoId={id} start={0} signedIn={signedIn} sample {...progress} />;
  return (
    <Plain signedIn={signedIn}>
      {progress.problem ? (
        <>
          <h1>{progress.problem}</h1>
          <button className="primary" type="button" onClick={progress.onTryAgain}>
            Try again
          </button>
        </>
      ) : (
        <p className="sub">Opening the sample lecture.</p>
      )}
    </Plain>
  );
}

function Plain({ signedIn, children }: { signedIn?: boolean; children: ReactNode }) {
  return (
    <div className="page">
      <div className="glow" aria-hidden="true" />
      <div className="wrap">
        <Nav signedIn={signedIn} />
        <main className="home">{children}</main>
      </div>
    </div>
  );
}
