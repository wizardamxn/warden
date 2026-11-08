"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { needsLogin } from "@/lib/api/client";
import { getLibrary, getRevise, type LibraryNote, type ReviseCard } from "@/lib/api/notes";
import { useToLogin } from "@/lib/auth/session";
import { formatDay, formatTime } from "@/lib/time";
import { useHasSample } from "@/lib/useHasSample";
import { Nav } from "../ui/Nav";
import { Flashcards } from "./Flashcards";
import "./study.css";

// Matches reviseAfter in server/internal/jobs/library.go.
const REVISE_AFTER_MS = 2 * 24 * 60 * 60 * 1000;

// Cards waiting from lectures added more than 2 days ago, oldest lecture first: a pile, worked through one card at a time.
export function Revise() {
  const toLogin = useToLogin();
  const [cards, setCards] = useState<ReviseCard[]>();
  const [problem, setProblem] = useState("");

  useEffect(() => {
    getRevise().then(setCards, (failure) => (needsLogin(failure) ? toLogin() : setProblem(failure.message)));
  }, [toLogin]);

  const lectures = new Set(cards?.map(({ video_id }) => video_id)).size;

  return (
    <div className="page">
      <div className="glow" aria-hidden="true" />
      <div className="wrap">
        <Nav signedIn={cards ? true : undefined} />
        <main className="home revise">
          <h1>Revise</h1>
          {problem && <p className="problem">{problem}</p>}
          {cards?.length === 0 && <Nothing />}
          {cards && cards.length > 0 && (
            <>
              <p className="lead">
                {cards.length} {cards.length === 1 ? "card" : "cards"} waiting from {lectures} {lectures === 1 ? "lecture" : "lectures"} you added more than 2
                days ago.
              </p>
              <Flashcards
                cards={cards}
                active
                pile
                link={(card) => (
                  <p className="sub from">
                    From {card.title || "an untitled lecture"}, added {formatDay(card.added_at)}.{" "}
                    <Link className="watch" href={`/notes/${card.video_id}?t=${Math.floor(card.start_seconds)}`}>
                      Watch {formatTime(card.start_seconds)}
                    </Link>
                  </p>
                )}
                end={<p>That&apos;s the whole pile. More arrive as your lectures turn 2 days old.</p>}
              />
            </>
          )}
        </main>
      </div>
    </div>
  );
}

// An empty pile says when the next cards arrive, or how to make the first ones.
function Nothing() {
  const [empty, setEmpty] = useState(false);
  const [next, setNext] = useState<{ note: LibraryNote; at: number }>();
  const sample = useHasSample(empty);

  useEffect(() => {
    // Without the library this just says how Revise works.
    getLibrary().then((notes) => {
      setEmpty(notes.length === 0);
      // The soonest lecture still too new for Revise. A failed one never brings cards.
      const now = Date.now();
      setNext(
        notes
          .filter(({ status }) => status !== "failed")
          .map((note) => ({ note, at: new Date(note.added_at).getTime() + REVISE_AFTER_MS }))
          .filter(({ at }) => at > now)
          .sort((a, b) => a.at - b.at)[0],
      );
    }, () => {});
  }, []);

  return (
    <>
      <p className="lead">Nothing to revise yet. A lecture&apos;s flashcards arrive here 2 days after you add it, when coming back to them helps most.</p>
      {next ? (
        <p className="sub">
          The next ones, from {next.note.title || "an untitled lecture"}, arrive on {formatDay(new Date(next.at).toISOString())}.
        </p>
      ) : (
        <Link className="link" href="/">
          Paste a YouTube link
        </Link>
      )}
      {sample && (
        <p className="sub try">
          Or{" "}
          <Link className="link" href="/sample">
            open the sample lecture
          </Link>{" "}
          to try its flashcards now.
        </p>
      )}
    </>
  );
}
