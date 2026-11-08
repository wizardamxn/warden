"use client";

import { addTransitionType, startTransition, useEffect, useState, ViewTransition, type ReactNode } from "react";
import type { Flashcard } from "@/lib/api/notes";
import { isTyping } from "@/lib/keys";
import "./study.css";

// Next slides the card off to the left; Previous brings the last one back from there. Other changes don't animate.
const CARD_IN = { "card-next": "card-in-next", "card-back": "card-in-back", default: "none" };
const CARD_OUT = { "card-next": "card-out-next", "card-back": "card-out-back", default: "none" };

type Props<Card extends Flashcard> = {
  cards: Card[];
  // Only the view on screen listens to the keys.
  active: boolean;
  // What sits under the card: its Watch link, and in Revise the lecture it came from.
  link(card: Card): ReactNode;
  // Shown with the last card.
  end: ReactNode;
  // Revise: the cards still to come show as a pile under this one.
  pile?: boolean;
};

// One index card at a time. It turns over on click, Space or Enter; the arrow keys move between cards.
export function Flashcards<Card extends Flashcard>({ cards, active, link, end, pile }: Props<Card>) {
  const [index, setIndex] = useState(0);
  const [turned, setTurned] = useState(false);
  const card = cards[index];
  const last = index === cards.length - 1;

  function move(by: number) {
    const next = Math.min(Math.max(index + by, 0), cards.length - 1);
    if (next === index) return;
    // A transition, so the card on screen slides away and the next lifts off the deck (study.css).
    startTransition(() => {
      addTransitionType(by > 0 ? "card-next" : "card-back");
      setIndex(next);
      setTurned(false);
    });
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
      <div className={pile ? "stack pile" : "stack"} data-left={pile ? Math.min(cards.length - 1 - index, 2) : undefined}>
        <div className="top">
          {/* A new card each time (the key), so moving on never shows the next card's answer turning back over. */}
          <ViewTransition key={index} enter={CARD_IN} exit={CARD_OUT} default="none">
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
          </ViewTransition>
        </div>
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

      {link(card)}
      <p className="sub hint">Click the card or press Space to turn it over. The arrow keys move between cards.</p>

      {last && <div className="next">{end}</div>}
    </div>
  );
}
