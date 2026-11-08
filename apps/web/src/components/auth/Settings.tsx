"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { logout, setTemplate, type Template } from "@/lib/api/account";
import { needsLogin } from "@/lib/api/client";
import { useMe, useToLogin } from "@/lib/auth/session";
import { Nav } from "../ui/Nav";
import { ThemePicker } from "../ui/ThemePicker";
import { TEMPLATES, TemplatePicker } from "./TemplatePicker";
import "./auth.css";

export function Settings() {
  const router = useRouter();
  const toLogin = useToLogin();
  const { user, setUser, problem: down } = useMe(true);
  const [picked, setPicked] = useState<Template>();
  const [said, setSaid] = useState("");
  const [problem, setProblem] = useState("");
  const [waiting, setWaiting] = useState(false);
  const choice = picked ?? user?.template;

  async function save(event: FormEvent) {
    event.preventDefault();
    if (!choice) return;
    setWaiting(true);
    setSaid("");
    setProblem("");
    try {
      const saved = await setTemplate(choice);
      setUser(saved);
      setSaid(`Saved. Notes you make from now on use the ${TEMPLATES[saved.template].name} template.`);
    } catch (failure) {
      if (needsLogin(failure)) return toLogin();
      setProblem((failure as Error).message);
    } finally {
      setWaiting(false);
    }
  }

  async function leave() {
    setProblem("");
    try {
      await logout();
      router.replace("/");
    } catch (failure) {
      setProblem((failure as Error).message);
    }
  }

  return (
    <div className="page">
      <div className="glow" aria-hidden="true" />
      <div className="wrap">
        <Nav signedIn={user ? true : undefined} />
        <main className="home auth">
          <h1>Settings</h1>
          {down && <p className="problem">{down}</p>}
          {user && (
            <>
              <form onSubmit={save}>
                <TemplatePicker
                  legend="Your template"
                  value={choice}
                  onChange={(next) => {
                    setPicked(next);
                    setSaid("");
                  }}
                >
                  <p className="sub">
                    Switching only changes notes you make from now on. Notes already in your library keep the template they were made with.
                  </p>
                </TemplatePicker>
                <div className="status" aria-live="polite">
                  {said && <p className="sub">{said}</p>}
                  {problem && <p className="problem">{problem}</p>}
                </div>
                <button className="primary" type="submit" disabled={waiting || choice === user.template}>
                  Save template
                </button>
              </form>

              <ThemePicker />

              <div className="account">
                <p className="sub">Logged in as {user.email}.</p>
                <button className="quiet out" type="button" onClick={leave}>
                  Log out
                </button>
              </div>
            </>
          )}
        </main>
      </div>
    </div>
  );
}
