"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useRef, useState, type FormEvent } from "react";
import { login, signup, type Template } from "@/lib/api/account";
import { ApiError } from "@/lib/api/client";
import { nextPath } from "@/lib/auth/session";
import { Nav } from "../ui/Nav";
import { TemplatePicker } from "./TemplatePicker";
import "./auth.css";

type Errors = { email?: string; password?: string; template?: string; form?: string };

// Log in or sign up, then go back to `next`. Form values are only ever sent to the API, never logged or stored.
export function AuthForm({ mode, next: asked }: { mode: "login" | "signup"; next?: string }) {
  const router = useRouter();
  const next = nextPath(asked);
  const joining = mode === "signup";
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [template, setTemplate] = useState<Template>();
  const [errors, setErrors] = useState<Errors>({});
  const [waiting, setWaiting] = useState(false);
  const emailInput = useRef<HTMLInputElement>(null);
  const passwordInput = useRef<HTMLInputElement>(null);
  const backTo = next === "/" ? "" : `?next=${encodeURIComponent(next)}`;

  function check(): Errors {
    const found: Errors = {};
    if (!email.includes("@")) found.email = "Enter your email address.";
    if (!password) found.password = "Enter your password.";
    else if (joining && password.length < 8) found.password = "Use at least 8 characters.";
    else if (joining && password.length > 128) found.password = "Use at most 128 characters.";
    if (joining && !template) found.template = "Pick a template.";
    return found;
  }

  async function submit(event: FormEvent) {
    event.preventDefault();
    const found = check();
    setErrors(found);
    if (found.email) return emailInput.current?.focus();
    if (found.password) return passwordInput.current?.focus();
    if (found.template) return (document.querySelector('input[name="template"]') as HTMLInputElement | null)?.focus();

    setWaiting(true);
    try {
      await (joining && template ? signup(email, password, template) : login(email, password));
      router.replace(next);
    } catch (problem) {
      const message = problem instanceof Error ? problem.message : "Something went wrong. Try again.";
      // 409: that email already has an account. Every other answer is about the whole form.
      setErrors(problem instanceof ApiError && problem.status === 409 ? { email: message } : { form: message });
      setWaiting(false);
    }
  }

  // Each field points at its own error and at the form's, so a screen reader hears both.
  const describe = (...ids: (string | false | undefined)[]) => ids.filter(Boolean).join(" ") || undefined;

  return (
    <div className="page">
      <div className="glow" aria-hidden="true" />
      <div className="wrap">
        <Nav />
        <main className="home auth">
          <h1>{joining ? "Make a Warden account" : "Log in to Warden"}</h1>
          {joining && <p className="sub">Free. You can make notes from 3 lectures a day.</p>}

          <form onSubmit={submit} noValidate>
            <div className="field">
              <label htmlFor="email">Email</label>
              <input
                id="email"
                ref={emailInput}
                className="line"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                aria-invalid={Boolean(errors.email || errors.form)}
                aria-describedby={describe(errors.email && "email-error", errors.form && "form-error")}
              />
              {errors.email && (
                <span className="problem" id="email-error">
                  {errors.email}
                </span>
              )}
            </div>

            <div className="field">
              <label htmlFor="password">Password</label>
              <input
                id="password"
                ref={passwordInput}
                className="line"
                type="password"
                autoComplete={joining ? "new-password" : "current-password"}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                aria-invalid={Boolean(errors.password || errors.form)}
                aria-describedby={describe(joining && "password-hint", errors.password && "password-error", errors.form && "form-error")}
              />
              {joining && (
                <span className="sub" id="password-hint">
                  8 to 128 characters.
                </span>
              )}
              {errors.password && (
                <span className="problem" id="password-error">
                  {errors.password}
                </span>
              )}
            </div>

            {joining && (
              <TemplatePicker legend="Template" value={template} onChange={setTemplate} error={errors.template}>
                <p className="sub">Sets how your notes and quiz are made. You can switch it later in settings.</p>
              </TemplatePicker>
            )}

            <div className="status" aria-live="polite">
              {errors.form && (
                <p className="problem" id="form-error">
                  {errors.form}
                </p>
              )}
            </div>

            <div className="join">
              <button className="primary" type="submit" disabled={waiting}>
                {joining ? "Sign up" : "Log in"}
              </button>
              <p className="sub">
                {joining ? "Already have an account? " : "No account yet? "}
                <Link className="link" href={`${joining ? "/login" : "/signup"}${backTo}`}>
                  {joining ? "Log in" : "Sign up"}
                </Link>
              </p>
            </div>
          </form>
        </main>
      </div>
    </div>
  );
}
