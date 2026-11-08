import type { ReactNode } from "react";
import type { Template } from "@/lib/api/account";
import "./auth.css";

export const TEMPLATES: Record<Template, { name: string; about: string }> = {
  general: {
    name: "General",
    about: "Any subject or exam. Notes, flashcards and a quiz of plain questions, scored as how many you got right.",
  },
  neet: {
    name: "NEET",
    about: "NEET lectures. Notes name the NCERT chapter, and the quiz uses NEET's question types, marked +4 and −1.",
  },
};

type Props = {
  legend: string;
  value: Template | undefined;
  onChange(template: Template): void;
  error?: string;
  children?: ReactNode;
};

// Two bubbles, filled in blue pen like the answer sheet. `children` is a line of help under the legend.
export function TemplatePicker({ legend, value, onChange, error, children }: Props) {
  return (
    <fieldset className="templates" aria-describedby={error ? "template-error" : undefined}>
      <legend>{legend}</legend>
      {children}
      {(Object.keys(TEMPLATES) as Template[]).map((id) => (
        <label className="choice" key={id}>
          <input className="sr" type="radio" name="template" value={id} checked={value === id} onChange={() => onChange(id)} />
          <span className="bubble" />
          <span>
            <span className="name">{TEMPLATES[id].name}</span>
            <span className="sub">{TEMPLATES[id].about}</span>
          </span>
        </label>
      ))}
      {error && (
        <p className="problem" id="template-error">
          {error}
        </p>
      )}
    </fieldset>
  );
}
