import type { Page, Route } from "@playwright/test";
import fixture from "../fixtures/k7Zb1iOV-ys.json";

// Stand-ins for the Go API (docs/API.md) and YouTube, so no test calls a real server, Gemini or a worker.

export { fixture };
export const VIDEO_ID = "k7Zb1iOV-ys";

// A stand-in for YouTube's script, so these tests don't depend on YouTube. The tests drive it through window.fakePlayer.
export const FAKE_API = `
  window.YT = {
    Player: class {
      constructor(element, options) {
        element.replaceWith(Object.assign(document.createElement("iframe"), { title: "fake player" }));
        this.time = 0;
        this.seeks = [];
        this.options = options;
        window.fakePlayer = this;
        window.playersMade = (window.playersMade ?? 0) + 1;
        setTimeout(() => options.events.onReady());
      }
      getCurrentTime() { return this.time; }
      seekTo(seconds) { this.seeks.push(seconds); this.time = seconds; }
      playVideo() {}
      destroy() {}
    },
  };
  window.onYouTubeIframeAPIReady();
`;

export type FakePlayer = {
  time: number;
  seeks: number[];
  options: { host: string; events: { onError(event: { data: number }): void } };
};
export const fakePlayer = (page: Page) => page.evaluateHandle(() => (window as never as { fakePlayer: FakePlayer }).fakePlayer);

export type Status = "pending" | "running" | "done" | "failed";

// What the API answers, built from the job status and the four step statuses in order.
export function job(status: string, steps: Status[], note: object | null = null, error = "") {
  const names = ["captions", "notes", "flashcards", "quiz"];
  return { video_id: VIDEO_ID, status, steps: steps.map((step, index) => ({ name: names[index], status: step, error: "" })), note, error };
}

export const USER = { id: 12, email: "asha@example.com", template: "neet", notes_today: 1, daily_limit: 3 };

// The session cookie the real API sets on login. Tests that care about cookies set it with logIn.
export const SESSION = "warden_session";

// GET /v1/me answers the user only when the request carries the session cookie, like the real API.
// The cookie only arrives if the website sends `credentials: "include"`, so this also checks that.
export async function mockMe(page: Page, user: object = USER) {
  const me = { user };
  // Registered first so every other route wins: anything a test didn't mock is refused, never sent to a real API on 8080.
  await page.route("http://127.0.0.1:8080/**", (route) => route.abort());
  // Lecture stills come from YouTube; tests don't fetch them.
  await page.route("https://i.ytimg.com/**", (route) => route.abort());
  await page.route("**/v1/me", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const cookie = (await route.request().allHeaders()).cookie ?? "";
    if (!cookie.includes(`${SESSION}=`)) return route.fulfill({ status: 401, json: { error: "Log in to continue." } });
    return route.fulfill({ json: { user: me.user } });
  });
  await logIn(page);
  return me;
}

// What the API's login does: puts its HttpOnly cookie on the API's host.
export async function logIn(page: Page) {
  await page.context().addCookies([{ name: SESSION, value: "token", domain: "127.0.0.1", path: "/", httpOnly: true, sameSite: "Lax" }]);
}

export async function logOut(page: Page) {
  await page.context().clearCookies();
}

// A login-needed route that answers 401 without the session cookie, like the real API.
export async function guarded(route: Route, json: object) {
  const cookie = (await route.request().allHeaders()).cookie ?? "";
  return cookie.includes(`${SESSION}=`) ? route.fulfill({ json }) : route.fulfill({ status: 401, json: { error: "Log in to continue." } });
}
