"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { getLibrary, getRevise, type LibraryNote } from "@/lib/api/notes";
import type { User } from "@/lib/api/account";
import { useMe } from "@/lib/auth/session";
import { useHasSample } from "@/lib/useHasSample";
import { Shelf } from "../library/Shelf";
import { Nav } from "../ui/Nav";
import { PasteLink } from "./PasteLink";
import "./notes.css";

const RECENT = 3;

// Logged out: what Warden does and the sample. Logged in: the paste box and today's count, then what's waiting.
export function Home() {
  const { user, problem } = useMe(false);
  const full = user ? user.notes_today >= user.daily_limit : false;
  // The sample link shows only once the API has a sample to open.
  const sample = useHasSample(user === null);

  return (
    <div className="page">
      <div className="glow" aria-hidden="true" />
      <div className="wrap">
        <Nav signedIn={user ? true : undefined} />
        <main className="home">
          {problem ? (
            <h1>{problem}</h1>
          ) : user === null ? (
            <>
              <h1>Notes beside the video, then test yourself.</h1>
              <p className="lead">
                Paste any YouTube lecture. Warden writes notes that follow the video as it plays, then flashcards and a quiz from the same lecture.
              </p>
              <div className="join">
                <Link className="primary" href="/signup">
                  Sign up
                </Link>
                <Link className="link" href="/login">
                  Log in
                </Link>
              </div>
              {sample && (
                <p className="sub try">
                  <Link className="link" href="/sample">
                    Open a sample lecture
                  </Link>{" "}
                  to see it first. No account needed.
                </p>
              )}
            </>
          ) : user && full ? (
            <>
              <h1>That&apos;s your {user.daily_limit} notes for today.</h1>
              <p className="lead">You can make more tomorrow: the count starts again at midnight, India time.</p>
              <Desk user={user} />
            </>
          ) : user ? (
            <>
              <h1>Paste a YouTube link.</h1>
              <p className="lead">Get notes beside the video, with every section tied to its moment in the lecture.</p>
              <PasteLink />
              <p className="sub count">
                {user.notes_today} of {user.daily_limit} notes today
              </p>
              <Desk user={user} />
            </>
          ) : null}
        </main>
      </div>
    </div>
  );
}

// What's already on the student's desk: cards waiting in Revise, and the last few lectures.
// Either part just stays away if it can't be loaded; the paste box above still works.
function Desk({ user }: { user: User }) {
  const [waiting, setWaiting] = useState<{ cards: number; lectures: number }>();
  const [recent, setRecent] = useState<LibraryNote[]>();
  const [more, setMore] = useState(false);

  useEffect(() => {
    getRevise().then(
      (cards) => setWaiting({ cards: cards.length, lectures: new Set(cards.map(({ video_id }) => video_id)).size }),
      () => {},
    );
    getLibrary().then((notes) => {
      setRecent(notes.slice(0, RECENT));
      setMore(notes.length > RECENT);
    }, () => {});
  }, []);

  if (!waiting?.cards && !recent?.length) return null;
  return (
    <div className="desk">
      {waiting && waiting.cards > 0 && (
        <div className="waiting-cards">
          <p>
            {waiting.cards} {waiting.cards === 1 ? "card is" : "cards are"} waiting in Revise, from {waiting.lectures}{" "}
            {waiting.lectures === 1 ? "lecture" : "lectures"} you added more than 2 days ago.
          </p>
          <Link className="link" href="/revise">
            Start revising
          </Link>
        </div>
      )}
      {recent && recent.length > 0 && (
        <section aria-labelledby="recent">
          <h2 className="label" id="recent">
            Your last {recent.length === 1 ? "lecture" : "lectures"}
          </h2>
          <Shelf notes={recent} template={user.template} />
          {more && (
            <Link className="link" href="/library">
              All of your library
            </Link>
          )}
        </section>
      )}
    </div>
  );
}
