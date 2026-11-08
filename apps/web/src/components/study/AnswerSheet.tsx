"use client";

import { useEffect, useId, useRef, useState, type CSSProperties } from "react";
import type { Template } from "@/lib/api/account";
import type { Question } from "@/lib/api/notes";
import { Watch } from "./Watch";
import "./study.css";

type Props = {
  quiz: Question[];
  template: Template;
  videoId: string;
  onWatch: ((seconds: number) => void) | null;
};

type Result = "right" | "wrong" | "blank";

// NEET: marked like the real paper, +4, −1 and 0. General: a teacher's tick or cross, and the score is how many are right.
const MARKS: Record<Template, Record<Result, string>> = {
  neet: { right: "+4", wrong: "−1", blank: "0" },
  general: { right: "✓", wrong: "✗", blank: "–" },
};

// NEET prints a match question's two lists side by side. Anything else, or lists that don't pair up, stays as its own lines.
function matchLists(text: string) {
  const lines = text.split("\n").map((line) => line.trim());
  const one = lines.findIndex((line) => /^list[ -]?i:?$/i.test(line));
  const two = lines.findIndex((line) => /^list[ -]?ii:?$/i.test(line));
  if (one < 0 || two < one) return null;
  const left = lines.slice(one + 1, two).filter(Boolean);
  const right = lines.slice(two + 1).filter(Boolean);
  if (left.length === 0 || left.length !== right.length) return null;
  return { intro: lines.slice(0, one).join("\n"), left, right };
}

function QuestionText({ question }: { question: Question }) {
  const lists = question.type === "match" ? matchLists(question.question) : null;
  if (!lists) return <span className="words">{question.question}</span>;
  return (
    <>
      <span className="words">{lists.intro}</span>
      <table className="lists">
        <thead>
          <tr>
            <th scope="col">List I</th>
            <th scope="col">List II</th>
          </tr>
        </thead>
        <tbody>
          {lists.left.map((item, index) => (
            <tr key={index}>
              <td>{item}</td>
              <td>{lists.right[index]}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}

// A negative NEET score is written with a real minus sign.
function signed(score: number): string {
  return String(score).replace("-", "−");
}

// The whole quiz on one OMR sheet: fill bubbles, submit once, then see every mark and why.
export function AnswerSheet({ quiz, template, videoId, onWatch }: Props) {
  const neet = template === "neet";
  const [answers, setAnswers] = useState<(number | undefined)[]>([]);
  const [marked, setMarked] = useState(false);
  const id = useId();
  const head = useRef<HTMLHeadingElement>(null);
  const stroke = useRef<HTMLSpanElement>(null);
  const glow = useRef<HTMLSpanElement>(null);
  const shownScore = useRef<HTMLSpanElement>(null);
  // Set by Submit and Try again, so opening the page doesn't move it.
  const moved = useRef(false);

  const results = quiz.map((question, index): Result =>
    answers[index] === undefined ? "blank" : answers[index] === question.answer_index ? "right" : "wrong",
  );
  const count = (result: string) => results.filter((each) => each === result).length;
  const score = neet ? count("right") * 4 - count("wrong") : count("right");
  const filled = answers.filter((answer) => answer !== undefined).length;

  function fill(index: number, option: number | undefined) {
    const next = [...answers];
    next[index] = option;
    setAnswers(next);
  }

  // Submit (true) or start again (false).
  function mark(on: boolean) {
    moved.current = true;
    if (!on) setAnswers([]);
    setMarked(on);
  }

  useEffect(() => {
    if (!moved.current) return;
    head.current?.scrollIntoView({ block: "center" });
    head.current?.focus({ preventScroll: true });
    // The one small celebration: the highlighter sweeps under the score as the light arrives. Once per submit.
    if (!marked || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const ease = { duration: 400, easing: "ease-out" };
    stroke.current?.animate([{ transform: "scaleX(0) rotate(-0.4deg)" }, { transform: "scaleX(1) rotate(-0.4deg)" }], { ...ease, delay: 150, fill: "backwards" });
    glow.current?.animate([{ opacity: 0 }, { opacity: 1 }], ease);
    // The score counts up to itself as the light arrives. Only the shown number moves; screen readers get the final one.
    const shown = shownScore.current;
    if (!shown) return;
    const began = performance.now();
    let frame = requestAnimationFrame(function tick(now) {
      const done = Math.min((now - began) / 500, 1);
      shown.textContent = signed(Math.round(score * (1 - (1 - done) ** 3)));
      if (done < 1) frame = requestAnimationFrame(tick);
    });
    return () => cancelAnimationFrame(frame);
    // Once per submit: the score can't change while the sheet is marked.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [marked]);

  return (
    <div className={marked ? "sheet marked" : "sheet"}>
      <div className="sheet-head">
        {marked ? (
          <>
            <span className="score-glow" ref={glow} aria-hidden="true" />
            <h2 className="score" ref={head} tabIndex={-1}>
              <span className="stroke" ref={stroke} />
              <span className="tx">
                <span ref={shownScore} aria-hidden="true">
                  {signed(score)}
                </span>
                <span className="sr">{signed(score)}</span> out of {neet ? quiz.length * 4 : quiz.length}
              </span>
            </h2>
            <p className="sub">
              {count("right")} right, {count("wrong")} wrong, {count("blank")} left blank.
            </p>
          </>
        ) : (
          <>
            <h2 ref={head} tabIndex={-1}>
              Answer sheet
            </h2>
            <p className="sub">
              {neet
                ? "Fill one bubble for each question. +4 for right, −1 for wrong, 0 if you leave it blank."
                : "Fill one bubble for each question. Your score is how many you get right."}
            </p>
          </>
        )}
      </div>

      <div className="omr">
        <div className="track" aria-hidden="true" />
        {quiz.map((question, index) => {
          const picked = answers[index];
          const result = results[index];
          return (
            <div className="q" key={index} style={{ "--i": index } as CSSProperties}>
              <div className="qtext" id={`${id}-${index}`}>
                <span className="qnum">{index + 1}</span>
                <div className="qbody">
                  <QuestionText question={question} />
                </div>
              </div>

              {marked ? (
                <p className="mark">
                  {MARKS[template][result]} <span>{result}</span>
                </p>
              ) : (
                picked !== undefined && (
                  <button className="quiet clear" type="button" aria-label={`Clear question ${index + 1}`} onClick={() => {
                      fill(index, undefined);
                      // The button disappears once the answer is blank, so keep keyboard users on this question.
                      (document.getElementsByName(`${id}-${index}`)[0] as HTMLInputElement | undefined)?.focus();
                    }}
                  >
                    Clear
                  </button>
                )
              )}

              <div className="opts" role="radiogroup" aria-labelledby={`${id}-${index}`}>
                {question.options.map((option, choice) => {
                  const right = marked && choice === question.answer_index;
                  const wrong = marked && choice === picked && !right;
                  return (
                    <label className={right ? "opt right" : wrong ? "opt wrong" : "opt"} key={choice}>
                      <input
                        className="sr"
                        type="radio"
                        name={`${id}-${index}`}
                        checked={picked === choice}
                        disabled={marked}
                        onChange={() => fill(index, choice)}
                      />
                      <span className="bubble">{choice + 1}</span>
                      <span className="tx">{option}</span>
                      {(right || wrong) && (
                        <span className="verdict">{wrong ? "Your answer, wrong" : picked === choice ? "Your answer, right" : "Right answer"}</span>
                      )}
                    </label>
                  );
                })}
              </div>

              {marked && (
                <div className="why">
                  <p>{question.explanation}</p>
                  <Watch videoId={videoId} seconds={question.start_seconds} onWatch={onWatch} />
                </div>
              )}
            </div>
          );
        })}
      </div>

      <div className="sheet-foot">
        {marked ? (
          <button className="primary" type="button" onClick={() => mark(false)}>
            Try the sheet again
          </button>
        ) : (
          <>
            <button className="primary" type="button" onClick={() => mark(true)}>
              Submit the sheet
            </button>
            <p className="sub">
              {filled} of {quiz.length} filled
            </p>
          </>
        )}
      </div>
    </div>
  );
}
