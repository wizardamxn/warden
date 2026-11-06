"use client";

import { useEffect, useState } from "react";
import type { Flashcard } from "@/lib/api/notes";
import { isTyping } from "@/lib/keys";
import { Watch } from "./Watch";
import "./study.css";

type Props = {
  cards: Flashcard[];
  // Only the view on screen listens to the keys.
  active: boolean;
  videoId: string;
  onWatch: ((seconds: number) => void) | null;
  onDone(): void;
};

// One index card at a time. It turns over on click, Space or Enter; the arrow keys move between cards.
export function Flashcards({ cards, active, videoId, onWatch, onDone }: Props) {
  const [index, setIndex] = useState(0);
  const [turned, setTurned] = useState(false);
  const card = cards[index];
  const last = index === cards.length - 1;

  function move(by: number) {
    const next = Math.min(Math.max(index + by, 0), cards.length - 1);
    if (next === index) return;
    setIndex(next);
    setTurned(false);
  }

  useEffect(() => {
    if (!active) return;
    function onKey(event: KeyboardEvent) {
      if (event.ctrlKey || event.metaKey || event.altKey || isTyping(event.target)) return;
      if (event.key === "ArrowRight") move(1);
      else if (event.key === "ArrowLeft") move(-1);
      // A focused button or link already answers Space and Enter itself.
      else if ((event.key === " " || event.key === "Enter") && !(event.target as HTMLElement).closest?.("button, a")) {
        event.preventDefault();
        setTurned((on) => !on);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  return (
    <div className="cards">
      <div className="stack">
        <button className={turned ? "card turned" : "card"} type="button" onClick={() => setTurned(!turned)}>
          <span className="face" aria-hidden={turned}>
            <span className="side">Question</span>
            <span className="words">{card.question}</span>
          </span>
          <span className="face back" aria-hidden={!turned}>
            <span className="side">Answer</span>
            <span className="words">{card.answer}</span>
          </span>
        </button>
      </div>

      <div className="turn">
        <button className="quiet" type="button" aria-disabled={index === 0} onClick={() => move(-1)}>
          Previous
        </button>
        <p className="sub" aria-live="polite">
          {index + 1} of {cards.length}
        </p>
        <button className="quiet" type="button" aria-disabled={last} onClick={() => move(1)}>
          Next
        </button>
      </div>
      <p className="sr" aria-live="polite">
        {turned ? `Answer: ${card.answer}` : ""}
      </p>

      <Watch videoId={videoId} seconds={card.start_seconds} onWatch={onWatch} />
      <p className="sub hint">Click the card or press Space to turn it over. The arrow keys move between cards.</p>

      {last && (
        <div className="next">
          <p>That&apos;s all {cards.length} cards.</p>
          <button className="primary" type="button" onClick={onDone}>
            Open the answer sheet
          </button>
        </div>
      )}
    </div>
  );
}
