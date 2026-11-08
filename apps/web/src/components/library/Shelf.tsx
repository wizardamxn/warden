import Link from "next/link";
import type { Template } from "@/lib/api/account";
import type { LibraryNote } from "@/lib/api/notes";
import { formatDay, formatTime } from "@/lib/time";
import { thumbnailUrl } from "@/lib/youtube";
import { TEMPLATES } from "../auth/TemplatePicker";
import { Lecture } from "../ui/Arrive";
import "./library.css";

// Said quietly beside each note. A finished note says nothing.
const STATUS: Record<LibraryNote["status"], string> = {
  queued: "In the queue",
  running: "Being made",
  done: "",
  partial: "Flashcards or quiz missing",
  failed: "Couldn't be made",
};

// Lectures as a list: the day hangs in the left margin, then the lecture's still and its title.
// `template` is the student's current one; a note made with another says so.
export function Shelf({ notes, template }: { notes: LibraryNote[]; template?: Template }) {
  const named = new Set<string>();
  return (
    <ol className="shelf">
      {notes.map((note) => {
        const href = `/notes/${note.video_id}`;
        const quiet = [
          note.channel && `${note.channel}, ${formatTime(note.duration_seconds)}`,
          STATUS[note.status],
          template && note.template !== template && `${TEMPLATES[note.template]?.name ?? note.template} template`,
        ].filter(Boolean);
        // The same video under two templates is two rows; only the first may carry the shared name.
        const first = !named.has(note.video_id);
        named.add(note.video_id);
        return (
          <li key={`${note.video_id}-${note.template}`}>
            <time className="day" dateTime={note.added_at}>
              {formatDay(note.added_at)}
            </time>
            {/* The title link is the one for keyboards and screen readers; the still is a second way in for the mouse. */}
            <Link className="still" href={href} tabIndex={-1} aria-hidden="true">
              <Lecture videoId={first ? note.video_id : null}>
                {/* A plain img: next/image would proxy YouTube's still through this site for no gain. */}
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={thumbnailUrl(note.video_id)} alt="" width={160} height={90} loading="lazy" />
              </Lecture>
            </Link>
            <div>
              <h2>
                <Link href={href}>{note.title || "Untitled lecture"}</Link>
              </h2>
              {quiet.length > 0 && <p className="sub">{quiet.join(" · ")}</p>}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
