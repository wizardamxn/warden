"use client";

import { useState } from "react";

// Matches the script in app/layout.tsx, which applies it before the page paints.
const THEME_KEY = "warden-theme";

type Theme = "device" | "light" | "dark";

const THEMES: [Theme, string, string][] = [
  ["device", "Same as this device", "Light or dark, following your device's setting."],
  ["light", "Light", "Warm paper."],
  ["dark", "Dark", "The same desk at night."],
];

function saved(): Theme {
  try {
    const theme = localStorage.getItem(THEME_KEY);
    return theme === "light" || theme === "dark" ? theme : "device";
  } catch {
    return "device";
  }
}

// Sets the page's theme now and for the next visit.
function apply(theme: Theme) {
  const root = document.documentElement;
  if (theme === "device") delete root.dataset.theme;
  else root.dataset.theme = theme;
  try {
    if (theme === "device") localStorage.removeItem(THEME_KEY);
    else localStorage.setItem(THEME_KEY, theme);
  } catch {
    // Private browsing can refuse storage: the theme still applies until the page closes.
  }
}

// Three bubbles like the template choice. Kept in this browser only, so it works before the page loads anything.
// Only shown once the page has loaded in the browser, so reading localStorage here can't differ from the server's render.
export function ThemePicker() {
  const [theme, setTheme] = useState<Theme>(saved);

  function choose(next: Theme) {
    setTheme(next);
    apply(next);
  }

  return (
    <fieldset className="templates theme">
      <legend>Theme</legend>
      <p className="sub">For this browser only.</p>
      {THEMES.map(([id, name, about]) => (
        <label className="choice" key={id}>
          <input className="sr" type="radio" name="theme" value={id} checked={theme === id} onChange={() => choose(id)} />
          <span className="bubble" />
          <span>
            <span className="name">{name}</span>
            <span className="sub">{about}</span>
          </span>
        </label>
      ))}
    </fieldset>
  );
}
