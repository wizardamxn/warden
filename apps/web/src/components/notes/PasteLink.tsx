"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { makeNote } from "@/lib/api/notes";
import { parseVideoId } from "@/lib/youtube";
import "./notes.css";

export function PasteLink() {
  const router = useRouter();
  const [link, setLink] = useState("");
  const [error, setError] = useState("");
  const [waiting, setWaiting] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!parseVideoId(link)) {
      setError("That doesn't look like a YouTube video link. Try one like youtube.com/watch?v=...");
      return;
    }
    setError("");
    setWaiting(true);
    try {
      const job = await makeNote(link);
      router.push(`/notes/${job.video_id}`);
    } catch (problem) {
      setError(problem instanceof Error ? problem.message : "Something went wrong. Nothing was saved.");
      setWaiting(false);
    }
  }

  return (
    <>
      <form className="paste" onSubmit={submit} noValidate>
        <label>
          YouTube link
          <input
            type="url"
            value={link}
            onChange={(event) => setLink(event.target.value)}
            placeholder="https://www.youtube.com/watch?v=..."
            disabled={waiting}
            aria-invalid={Boolean(error)}
          />
        </label>
        <button className="primary" type="submit" disabled={waiting}>
          Make notes
        </button>
      </form>
      <div className="status" aria-live="polite">
        {waiting && <p className="sub">Sending the link.</p>}
        {error && <p className="problem">{error}</p>}
      </div>
    </>
  );
}
