import { ViewTransition, type ReactNode } from "react";

// Wraps each page: moving between pages fades the old one out and lets the new one rise into place (globals.css).
// Only navigations animate it, never the first load; browsers without view transitions just swap.
export function Arrive({ children }: { children: ReactNode }) {
  return (
    <ViewTransition enter="arrive" exit="leave" default="none">
      {children}
    </ViewTransition>
  );
}

// The same lecture's still in a list and its player on the note page share this name, so one grows into the other.
// A name may appear only once per page, so lists give it to a video's first row only.
export function Lecture({ videoId, children }: { videoId: string | null; children: ReactNode }) {
  if (!videoId) return children;
  return (
    <ViewTransition name={`video-${videoId}`} share="morph" default="none">
      {children}
    </ViewTransition>
  );
}
