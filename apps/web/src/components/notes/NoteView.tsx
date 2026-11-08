"use client";

import Link from "next/link";
import { useCallback, useEffect, useRef, useState } from "react";
import type { Section } from "@/lib/api/notes";
import { isTyping } from "@/lib/keys";
import { formatTime } from "@/lib/time";
import { thumbnailUrl, watchUrlAt } from "@/lib/youtube";
import { AnswerSheet } from "../study/AnswerSheet";
import { Flashcards } from "../study/Flashcards";
import { Watch } from "../study/Watch";
import { Lecture } from "../ui/Arrive";
import { Nav } from "../ui/Nav";
import { NotReady, StatusLine, Waiting, type ProgressProps } from "./Progress";
import { useYouTubePlayer } from "./useYouTubePlayer";

type View = "notes" | "cards" | "sheet";
const VIEWS: [View, string][] = [
  ["notes", "Notes"],
  ["cards", "Flashcards"],
  ["sheet", "Answer sheet"],
];

// The section the video is in: the last one that has started.
function playingIndex(sections: Section[], seconds: number): number {
  return sections.reduce((playing, section, index) => (seconds >= section.start_seconds ? index : playing), -1);
}

type Props = {
  videoId: string;
  // Seconds to open the video at, from a Watch link in Revise. 0 starts at the beginning.
  start: number;
  // Undefined while unknown. Shapes the top line only.
  signedIn?: boolean;
  sample?: boolean;
} & ProgressProps;

// Stays mounted from the first wait to the finished note, so polling and the note arriving never rebuild the player.
export function NoteView({ videoId, start, signedIn, sample, ...progress }: Props) {
  const { job } = progress;
  const note = job?.note ?? null;
  const hasNote = note !== null;
  const sections = note?.sections ?? [];
  const mnemonics = note?.mnemonics ?? [];
  const glossary = note?.glossary ?? [];
  const cards = note?.flashcards ?? [];
  const quiz = note?.quiz ?? [];
  const startWith: View = cards.length === 0 && quiz.length > 0 ? "sheet" : "cards";
  const finishedSteps = job?.steps.filter(({ status }) => status === "done").length ?? 0;

  const frame = useRef<HTMLDivElement>(null);
  const headings = useRef<(HTMLHeadingElement | null)[]>([]);
  const [time, setTime] = useState(start);
  const [focus, setFocus] = useState(false);
  // Only the right column changes with the view; the player is never rebuilt.
  const [view, setView] = useState<View>("notes");
  const shownView = useRef(view);

  // Whole seconds, so the page redraws only when the second changes.
  const onTime = useCallback((seconds: number) => setTime(Math.floor(seconds)), []);
  const { blocked, seek } = useYouTubePlayer(frame, videoId, onTime);

  // The player seeks once it's ready; the highlight starts there straight away.
  useEffect(() => {
    if (start) seek(start);
  }, [start, seek]);

  const live = blocked ? -1 : playingIndex(sections, time);
  const focusOn = focus && !blocked && hasNote && view === "notes";

  function show(next: View) {
    setView(next);
    setFocus(false);
  }

  // "Watch 05:40" on a card or question: the same player jumps there and the notes come back at that moment.
  const watch = blocked
    ? null
    : (seconds: number) => {
        seek(seconds);
        setTime(Math.floor(seconds));
        show("notes");
      };

  useEffect(() => {
    const switched = shownView.current !== view;
    shownView.current = view;
    if (view === "notes" && live >= 0) {
      const calm = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      headings.current[live]?.scrollIntoView({ block: "start", behavior: calm || switched ? "auto" : "smooth" });
    } else if (switched) {
      window.scrollTo(0, 0);
    }
  }, [live, view]);

  useEffect(() => {
    if (!hasNote || view !== "notes") return;
    function onKey(event: KeyboardEvent) {
      if (event.key.toLowerCase() !== "f" || event.repeat || event.ctrlKey || event.metaKey || event.altKey) return;
      if (!isTyping(event.target)) setFocus((on) => !on);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [hasNote, view]);

  return (
    <div className={focusOn ? "page focus" : "page"}>
      {/* Each of the four finished steps brings the light closer, up to its full size. */}
      {(job || progress.problem) && <div className="glow" style={{ transform: `scale(${0.6 + finishedSteps * 0.1})` }} aria-hidden="true" />}
      <div className="wrap">
        <Nav
          signedIn={signedIn}
          end={
            hasNote &&
            !blocked &&
            view === "notes" && (
              <button className="quiet" type="button" aria-keyshortcuts="f" aria-pressed={focusOn} onClick={() => setFocus(!focus)}>
                {focusOn ? "Leave focus (F)" : "Focus (F)"}
              </button>
            )
          }
        >
          {hasNote &&
            VIEWS.map(([name, label]) => (
              <button className="quiet view" type="button" key={name} aria-current={view === name} onClick={() => show(name)}>
                {label}
              </button>
            ))}
        </Nav>

        <main className="cols">
          <div className="left">
            {!blocked && (
              <Lecture videoId={videoId}>
                {/* YouTube's still shows until its player covers it, so the library's still has somewhere to grow into. */}
                <div className="player" ref={frame} style={{ backgroundImage: `url(${thumbnailUrl(videoId)})` }} />
              </Lecture>
            )}
            <div className="lecture">
              {note && (
                <>
                  <h1>{note.title}</h1>
                  <p className="sub">
                    {note.channel}, {formatTime(note.duration_seconds)}
                  </p>
                  {note.ncert_chapter && <p className="sub">NCERT chapter: {note.ncert_chapter}</p>}
                  {sample && signedIn === false && (
                    <p className="sub">
                      A sample lecture.{" "}
                      <Link className="link" href="/signup">
                        Sign up
                      </Link>{" "}
                      to make notes from your own.
                    </p>
                  )}
                  <StatusLine {...progress} />
                </>
              )}
              {blocked && (
                <p className="blocked">
                  This video can&apos;t play here. Each timestamp opens it on{" "}
                  <a className="link" href={watchUrlAt(videoId, 0)} target="_blank" rel="noopener noreferrer">
                    YouTube
                  </a>{" "}
                  at that second.
                </p>
              )}
            </div>
          </div>

          {/* Views are hidden, not removed, so the scroll targets and each view's place survive switching. */}
          <div>
            <div hidden={view !== "notes"}>
              {!note && <Waiting {...progress} />}
              {note && <p className="tldr rest">{note.tldr}</p>}
              {note && (
                <ul className="points rest">
                  {(note.key_points ?? []).map((point, index) => (
                    <li key={index}>{point}</li>
                  ))}
                </ul>
              )}

              {sections.map((section, index) => {
                const clock = formatTime(section.start_seconds);
                return (
                  <section className="sec" key={index} aria-current={index === live ? "true" : undefined}>
                    <div>
                      {blocked ? (
                        <a
                          className="ts"
                          href={watchUrlAt(videoId, section.start_seconds)}
                          target="_blank"
                          rel="noopener noreferrer"
                          aria-label={`Open ${clock} on YouTube in a new tab`}
                        >
                          {clock}
                        </a>
                      ) : (
                        <button className="ts" type="button" aria-label={`Jump to ${clock}`} onClick={() => seek(section.start_seconds)}>
                          {clock}
                        </button>
                      )}
                      {section.exam_mark?.trim() && <p className="exam">{section.exam_mark}</p>}
                    </div>
                    <div>
                      <h2
                        ref={(heading) => {
                          headings.current[index] = heading;
                        }}
                      >
                        <span className="stroke" />
                        <span className="tx">{section.title}</span>
                      </h2>
                      <ul>
                        {(section.bullets ?? []).map((bullet, bulletIndex) => (
                          <li key={bulletIndex}>{bullet}</li>
                        ))}
                      </ul>
                    </div>
                  </section>
                );
              })}

              {mnemonics.length > 0 && (
                <div className="extra rest">
                  <h2>Mnemonics</h2>
                  <dl>
                    {mnemonics.map((mnemonic, index) => (
                      <div key={index}>
                        <dt>{mnemonic.topic}</dt>
                        <dd>{mnemonic.text}</dd>
                      </div>
                    ))}
                  </dl>
                </div>
              )}

              {glossary.length > 0 && (
                <div className="extra rest">
                  <h2>Glossary</h2>
                  <dl>
                    {glossary.map((entry, index) => (
                      <div key={index}>
                        <dt>{entry.term}</dt>
                        <dd>{entry.definition}</dd>
                      </div>
                    ))}
                  </dl>
                </div>
              )}

              {/* The end of the notes leads straight into testing yourself. */}
              {note && (
                <div className="next rest">
                  <p>Reading it isn&apos;t the same as knowing it.</p>
                  <button className="primary" type="button" onClick={() => show(startWith)}>
                    {startWith === "sheet" ? "Open the answer sheet" : "Start the flashcards"}
                  </button>
                </div>
              )}
            </div>

            {note && (
              <>
                <div hidden={view !== "cards"}>
                  {cards.length > 0 ? (
                    <Flashcards
                      cards={cards}
                      active={view === "cards"}
                      link={(card) => <Watch videoId={videoId} seconds={card.start_seconds} onWatch={watch} />}
                      end={
                        <>
                          <p>That&apos;s all {cards.length} cards.</p>
                          <button className="primary" type="button" onClick={() => show("sheet")}>
                            Open the answer sheet
                          </button>
                        </>
                      }
                    />
                  ) : (
                    <NotReady part="flashcards" {...progress} />
                  )}
                </div>
                <div hidden={view !== "sheet"}>
                  {quiz.length > 0 ? <AnswerSheet quiz={quiz} template={note.template ?? "neet"} videoId={videoId} onWatch={watch} /> : <NotReady part="quiz" {...progress} />}
                </div>
              </>
            )}
          </div>
        </main>
      </div>
      <div className="edges" aria-hidden="true" />
    </div>
  );
}
