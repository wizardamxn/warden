"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { needsLogin } from "@/lib/api/client";
import { getLibrary, type LibraryNote } from "@/lib/api/notes";
import { useMe, useToLogin } from "@/lib/auth/session";
import { useHasSample } from "@/lib/useHasSample";
import { Nav } from "../ui/Nav";
import { Shelf } from "./Shelf";

// The student's past notes, newest first, each dated in the margin like the timestamps in a note.
export function Library() {
  const toLogin = useToLogin();
  const { user } = useMe(true);
  const [notes, setNotes] = useState<LibraryNote[]>();
  const [problem, setProblem] = useState("");
  const empty = notes?.length === 0;
  const sample = useHasSample(empty);

  useEffect(() => {
    getLibrary().then(setNotes, (failure) => (needsLogin(failure) ? toLogin() : setProblem(failure.message)));
  }, [toLogin]);

  return (
    <div className="page">
      <div className="glow" aria-hidden="true" />
      <div className="wrap">
        <Nav signedIn={user ? true : undefined} />
        <main className="home library">
          <h1>Library</h1>
          {problem && <p className="problem">{problem}</p>}
          {empty && (
            <>
              <p className="lead">Your notes will be listed here.</p>
              <Link className="link" href="/">
                Paste a YouTube link to make your first
              </Link>
              {sample && (
                <p className="sub try">
                  Or{" "}
                  <Link className="link" href="/sample">
                    open the sample lecture
                  </Link>{" "}
                  to see what a finished note looks like.
                </p>
              )}
            </>
          )}
          {notes && notes.length > 0 && <Shelf notes={notes} template={user?.template} />}
        </main>
      </div>
    </div>
  );
}
