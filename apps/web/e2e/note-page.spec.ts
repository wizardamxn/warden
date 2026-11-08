import { expect, test, type Page } from "@playwright/test";
import { FAKE_API, fakePlayer, fixture, job, mockMe, VIDEO_ID } from "./mock";

const FINISHED = job("done", ["done", "done", "done", "done"], fixture);
const NOTE_PATH = `**/v1/notes/${VIDEO_ID}`;
const DOWN = "Warden's server isn't answering. Give it a minute.";

// A stand-in for the Go API. Set `api.answer` to change what the next poll sees, whichever poll that is.
async function mockApi(page: Page, answer: object) {
  const api = { answer, gets: 0, retries: 0 };
  await mockMe(page);
  await page.route("https://www.youtube.com/iframe_api", (route) => route.fulfill({ contentType: "text/javascript", body: FAKE_API }));
  await page.route(NOTE_PATH, (route) => {
    api.gets++;
    return route.fulfill({ json: api.answer });
  });
  await page.route(`${NOTE_PATH}/retry`, (route) => {
    api.retries++;
    return route.fulfill({ status: 202, json: api.answer });
  });
  return api;
}

async function openNote(page: Page, note: object = fixture) {
  const api = await mockApi(page, job("done", ["done", "done", "done", "done"], note));
  await page.goto(`/notes/${VIDEO_ID}`);
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Arjuna NEET");
  await page.waitForFunction("window.fakePlayer");
  return api;
}

async function openWaiting(page: Page, answer: object) {
  const api = await mockApi(page, answer);
  await page.goto(`/notes/${VIDEO_ID}`);
  await page.waitForFunction("window.fakePlayer");
  return api;
}

// Polling runs on a timer, so these wait out real polls.
async function twoMorePolls(api: { gets: number }) {
  const seen = api.gets;
  await expect.poll(() => api.gets, { timeout: 8000 }).toBeGreaterThanOrEqual(seen + 2);
}

async function expectNoMorePolls(page: Page, api: { gets: number }) {
  const seen = api.gets;
  await page.waitForTimeout(3000);
  expect(api.gets).toBe(seen);
}

const glowSize = (page: Page) => page.locator(".glow").evaluate((glow) => new DOMMatrix(getComputedStyle(glow).transform).a);
const playersMade = (page: Page) => page.evaluate("window.playersMade");

test("the saved note renders from the API", async ({ page }) => {
  await openNote(page);
  await expect(page.getByRole("heading", { name: "History: Robert Hooke" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Jump to 07:36" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Jump to 1:03:05" })).toBeVisible();
  await expect(page.getByText("NCERT chapter: Cell: The Unit of Life")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Mnemonics" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Glossary" })).toBeVisible();
  await expect(page.locator(".exam")).toHaveCount(0);
  await expect(page.getByText("Test yourself")).toHaveCount(0);
});

test("a note from an older server version renders without its newer fields", async ({ page }) => {
  const older: Record<string, unknown> = { ...fixture };
  for (const field of ["video_id", "ncert_chapter", "mnemonics", "flashcards", "quiz", "failed_steps"]) delete older[field];
  await openNote(page, older);
  await expect(page.getByRole("heading", { name: "History: Robert Hooke" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Mnemonics" })).toHaveCount(0);
  await expect(page.getByText("NCERT chapter")).toHaveCount(0);
});

test("clicking a timestamp seeks the player and the highlight follows the video", async ({ page }) => {
  await openNote(page);
  const hooke = page.locator(".sec", { hasText: "History: Robert Hooke" });
  await expect(page.locator(".sec[aria-current]")).toHaveCount(0);

  await page.getByRole("button", { name: "Jump to 44:05" }).click();
  await expect(hooke).toHaveAttribute("aria-current", "true");
  expect(await (await fakePlayer(page)).evaluate((player) => player.seeks)).toEqual([2645]);

  await (await fakePlayer(page)).evaluate((player) => (player.time = 1700));
  await expect(page.locator(".sec", { hasText: "Levels of Biological Organization" })).toHaveAttribute("aria-current", "true");
  await expect(hooke).not.toHaveAttribute("aria-current", "true");
});

test("the player is told to use the privacy-enhanced host", async ({ page }) => {
  await openNote(page);
  expect(await (await fakePlayer(page)).evaluate((player) => player.options.host)).toBe("https://www.youtube-nocookie.com");
});

test("focus mode works from the key and the button, but not while typing", async ({ page }) => {
  await openNote(page);
  const root = page.locator(".page");

  await page.keyboard.press("f");
  await expect(root).toHaveClass(/focus/);
  await page.getByRole("button", { name: "Leave focus (F)" }).click();
  await expect(root).not.toHaveClass(/focus/);

  await page.evaluate(() => document.body.append(Object.assign(document.createElement("input"), { id: "typing" })));
  await page.locator("#typing").focus();
  await page.keyboard.press("f");
  await expect(root).not.toHaveClass(/focus/);
});

for (const code of [101, 150]) {
  test(`error ${code} shows the notes alone with YouTube links`, async ({ page }) => {
    await openNote(page);
    await (await fakePlayer(page)).evaluate((player, data) => player.options.events.onError({ data }), code);

    await expect(page.locator(".player")).toHaveCount(0);
    await expect(page.getByRole("button", { name: /Focus/ })).toHaveCount(0);
    const link = page.getByRole("link", { name: "Open 07:36 on YouTube in a new tab" });
    await expect(link).toHaveAttribute("href", `https://www.youtube.com/watch?v=${VIDEO_ID}&t=456s`);
    await expect(link).toHaveAttribute("target", "_blank");
  });
}

test("an exam mark shows only where there is one, and AI text stays plain text", async ({ page }) => {
  const [first, ...rest] = fixture.sections;
  await openNote(page, {
    ...fixture,
    mnemonics: [{ topic: "Discoverers", text: "<img src=x onerror=alert(1)>" }],
    sections: [{ ...first, exam_mark: "Asked 2018, 2024" }, ...rest],
  });
  await expect(page.locator(".exam")).toHaveCount(1);
  await expect(page.locator(".exam")).toHaveText("Asked 2018, 2024");
  await expect(page.getByText("<img src=x onerror=alert(1)>")).toBeVisible();
  await expect(page.locator("main img")).toHaveCount(0);
});

test("pasting a link goes straight to the note page and starts polling", async ({ page }) => {
  const api = await mockApi(page, job("queued", ["pending", "pending", "pending", "pending"]));
  let sent: unknown;
  await page.route("**/v1/notes", (route) => {
    sent = route.request().postDataJSON();
    return route.fulfill({ status: 202, json: api.answer });
  });

  await page.goto("/");
  await page.getByLabel("YouTube link").fill(`https://youtu.be/${VIDEO_ID}?si=abc`);
  await page.getByRole("button", { name: "Make notes" }).click();

  await expect(page).toHaveURL(`/notes/${VIDEO_ID}`);
  expect(sent).toEqual({ url: `https://youtu.be/${VIDEO_ID}?si=abc` });
  await expect(page.getByRole("heading", { name: "In the queue." })).toBeVisible();
  await expect(page.locator(".player iframe")).toBeVisible();
  await twoMorePolls(api);
});

test("a link that's already finished opens its note", async ({ page }) => {
  await mockApi(page, FINISHED);
  await page.route("**/v1/notes", (route) => route.fulfill({ json: FINISHED }));
  await page.goto("/");
  await page.getByLabel("YouTube link").fill(`https://www.youtube.com/watch?v=${VIDEO_ID}`);
  await page.getByRole("button", { name: "Make notes" }).click();
  await expect(page.getByRole("heading", { name: "History: Robert Hooke" })).toBeVisible();
});

test("a bad link shows the API's message and stays on the form", async ({ page }) => {
  await mockMe(page);
  await page.route("**/v1/notes", (route) => route.fulfill({ status: 400, json: { error: "That video is private." } }));
  await page.goto("/");
  await page.getByLabel("YouTube link").fill(`https://youtu.be/${VIDEO_ID}`);
  await page.getByRole("button", { name: "Make notes" }).click();
  await expect(page.getByText("That video is private.")).toBeVisible();
  await expect(page).toHaveURL("/");
  await expect(page.getByRole("button", { name: "Make notes" })).toBeEnabled();
});

test("a link that isn't YouTube is refused without calling the API", async ({ page }) => {
  await mockMe(page);
  let calls = 0;
  await page.route("**/v1/notes", (route) => {
    calls++;
    return route.abort();
  });
  await page.goto("/");
  await page.getByLabel("YouTube link").fill("https://example.com/watch?v=abc");
  await page.getByRole("button", { name: "Make notes" }).click();
  await expect(page.getByText("That doesn't look like a YouTube video link.")).toBeVisible();
  expect(calls).toBe(0);
});

test("the API being down while pasting says so, and pressing the button again works", async ({ page }) => {
  await mockApi(page, FINISHED);
  await page.route("**/v1/notes", (route) => route.abort());
  await page.goto("/");
  await page.getByLabel("YouTube link").fill(`https://youtu.be/${VIDEO_ID}`);
  await page.getByRole("button", { name: "Make notes" }).click();
  await expect(page.getByText(DOWN)).toBeVisible();

  await page.unroute("**/v1/notes");
  await page.route("**/v1/notes", (route) => route.fulfill({ json: FINISHED }));
  await page.getByRole("button", { name: "Make notes" }).click();
  await expect(page).toHaveURL(`/notes/${VIDEO_ID}`);
});

test("an unknown video shows the way to make it, and polling stops", async ({ page }) => {
  const api = { gets: 0 };
  await page.route(NOTE_PATH, (route) => {
    api.gets++;
    return route.fulfill({ status: 404, json: { error: "unknown video" } });
  });
  await page.goto(`/notes/${VIDEO_ID}`);
  await expect(page.getByRole("heading", { name: "There's no note for this video yet." })).toBeVisible();
  await expect(page.getByRole("link", { name: "Paste its link to make one" })).toBeVisible();
  await expectNoMorePolls(page, api);
});

test("the words and the glow follow the real steps", async ({ page }) => {
  // Go can write an empty list as null, so a queued job may have no steps yet.
  const api = await openWaiting(page, { ...job("queued", []), steps: null });
  const heading = page.getByRole("heading", { level: 1 });
  await expect(heading).toHaveText("In the queue.");
  const atStart = await glowSize(page);

  api.answer = job("running", ["running", "pending", "pending", "pending"]);
  await expect(heading).toHaveText("Getting the captions.");
  expect(await glowSize(page)).toBe(atStart);

  api.answer = job("running", ["done", "running", "pending", "pending"]);
  await expect(heading).toHaveText("Writing the notes.");
  await expect.poll(() => glowSize(page)).toBeGreaterThan(atStart);
  await expect(page.getByRole("button", { name: /Focus/ })).toHaveCount(0);
});

test("the note appears while flashcards are still running, and they finish quietly", async ({ page }) => {
  const api = await openWaiting(page, job("running", ["done", "running", "pending", "pending"]));
  await expect(page.getByRole("heading", { name: "Writing the notes." })).toBeVisible();
  await page.locator(".player iframe").evaluate((frame) => frame.setAttribute("data-kept", "yes"));

  api.answer = job("running", ["done", "done", "running", "pending"], fixture);
  await expect(page.getByRole("heading", { name: "History: Robert Hooke" })).toBeVisible();
  await expect(page.getByText("Making the flashcards.")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Writing the notes." })).toHaveCount(0);
  await expect(page.locator(".player iframe[data-kept]")).toHaveCount(1);
  expect(await playersMade(page)).toBe(1);

  api.answer = FINISHED;
  await expect(page.getByText("Making the flashcards.")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Try again" })).toHaveCount(0);
});

test("polling doesn't touch the player, the scroll or the highlight", async ({ page }) => {
  const api = await openWaiting(page, job("running", ["done", "done", "running", "pending"], fixture));
  await page.getByRole("button", { name: "Jump to 44:05" }).click();
  const hooke = page.locator(".sec", { hasText: "History: Robert Hooke" });
  await expect(hooke).toHaveAttribute("aria-current", "true");
  await page.waitForTimeout(1000); // Lets the smooth scroll to the section settle.
  await page.locator(".player iframe").evaluate((frame) => frame.setAttribute("data-kept", "yes"));
  const scrolled = await page.evaluate(() => window.scrollY);
  expect(scrolled).toBeGreaterThan(0);

  await twoMorePolls(api);
  api.answer = job("running", ["done", "done", "done", "running"], fixture);
  await expect(page.getByText("Making the quiz.")).toBeVisible();

  expect(await page.evaluate(() => window.scrollY)).toBe(scrolled);
  await expect(hooke).toHaveAttribute("aria-current", "true");
  await expect(page.locator(".player iframe[data-kept]")).toHaveCount(1);
  expect(await playersMade(page)).toBe(1);
  expect(await (await fakePlayer(page)).evaluate((player) => player.seeks)).toEqual([2645]);
});

test("partial keeps the note usable, says what's missing, and Try again retries it", async ({ page }) => {
  const api = await openWaiting(page, job("partial", ["done", "done", "done", "failed"], fixture));
  await expect(page.getByText("The quiz couldn't be made.")).toBeVisible();
  await page.getByRole("button", { name: "Jump to 44:05" }).click();
  expect(await (await fakePlayer(page)).evaluate((player) => player.seeks)).toEqual([2645]);
  await expectNoMorePolls(page, api);

  api.answer = job("running", ["done", "done", "done", "running"], fixture);
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByText("Making the quiz.")).toBeVisible();
  expect(api.retries).toBe(1);
  await expect(page.getByText("couldn't be made")).toHaveCount(0);
  await expect(page.getByRole("heading", { name: "History: Robert Hooke" })).toBeVisible();

  api.answer = FINISHED;
  await expect(page.getByText("Making the quiz.")).toHaveCount(0);
  expect(await playersMade(page)).toBe(1);
});

test("partial names both parts when both failed", async ({ page }) => {
  await openWaiting(page, job("partial", ["done", "done", "failed", "failed"], fixture));
  await expect(page.getByText("The flashcards and the quiz couldn't be made.")).toBeVisible();
});

test("a failed note shows the plain error as text, and Try again retries it", async ({ page }) => {
  const error = "<img src=x onerror=alert(1)> has no captions.";
  const api = await openWaiting(page, job("failed", ["failed", "pending", "pending", "pending"], null, error));
  await expect(page.getByRole("heading", { name: "The notes couldn't be made." })).toBeVisible();
  await expect(page.getByText(error)).toBeVisible();
  await expect(page.locator("main img")).toHaveCount(0);
  await expect(page.locator(".player iframe")).toBeVisible();
  await expectNoMorePolls(page, api);

  api.answer = job("queued", ["pending", "pending", "pending", "pending"]);
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByRole("heading", { name: "In the queue." })).toBeVisible();
  expect(api.retries).toBe(1);

  api.answer = job("running", ["running", "pending", "pending", "pending"]);
  await expect(page.getByRole("heading", { name: "Getting the captions." })).toBeVisible();
  expect(await playersMade(page)).toBe(1);
});

test("a failed step's own error shows when the job has none", async ({ page }) => {
  const failed = job("failed", ["done", "failed", "pending", "pending"]);
  failed.steps[1].error = "The AI gave no usable answer.";
  await openWaiting(page, failed);
  await expect(page.getByText("The AI gave no usable answer.")).toBeVisible();
});

test("the API down on arrival gives a clear message, and Try again recovers", async ({ page }) => {
  const api = await mockApi(page, FINISHED);
  await page.unroute(NOTE_PATH);
  await page.route(NOTE_PATH, (route) => route.abort());
  await page.goto(`/notes/${VIDEO_ID}`);
  await expect(page.getByRole("heading", { name: DOWN })).toBeVisible({ timeout: 10000 });
  await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();

  await page.unroute(NOTE_PATH);
  await page.route(NOTE_PATH, (route) => route.fulfill({ json: FINISHED }));
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByRole("heading", { name: "History: Robert Hooke" })).toBeVisible();
  expect(api.retries).toBe(0);
});

test("one lost poll is forgiven, and the API going down keeps the note and the player", async ({ page }) => {
  const running = job("running", ["done", "done", "running", "pending"], fixture);
  const api = await openWaiting(page, running);
  await expect(page.getByRole("heading", { name: "History: Robert Hooke" })).toBeVisible();

  let polls = 0;
  await page.unroute(NOTE_PATH);
  await page.route(NOTE_PATH, (route) => (++polls === 1 ? route.abort() : route.fulfill({ json: running })));
  await expect.poll(() => polls, { timeout: 8000 }).toBeGreaterThanOrEqual(3);
  await expect(page.getByText(DOWN)).toHaveCount(0);

  await page.unroute(NOTE_PATH);
  await page.route(NOTE_PATH, (route) => route.abort());
  await expect(page.getByText(DOWN)).toBeVisible({ timeout: 10000 });
  await expect(page.getByRole("heading", { name: "History: Robert Hooke" })).toBeVisible();

  await page.unroute(NOTE_PATH);
  await page.route(NOTE_PATH, (route) => route.fulfill({ json: FINISHED }));
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByText(DOWN)).toHaveCount(0);
  await expect(page.getByText("Making the flashcards.")).toHaveCount(0);
  expect(api.retries).toBe(0);
  expect(await playersMade(page)).toBe(1);
});

test("a server error during polling shows the API's message", async ({ page }) => {
  await mockApi(page, FINISHED);
  await page.unroute(NOTE_PATH);
  await page.route(NOTE_PATH, (route) => route.fulfill({ status: 500, json: { error: "The database is restarting." } }));
  await page.goto(`/notes/${VIDEO_ID}`);
  await expect(page.getByRole("heading", { name: "The database is restarting." })).toBeVisible({ timeout: 10000 });
});

test("polling stops when the job finishes", async ({ page }) => {
  const api = await openWaiting(page, job("running", ["done", "done", "running", "pending"], fixture));
  await twoMorePolls(api);
  api.answer = FINISHED;
  await expect(page.getByText("Making the flashcards.")).toHaveCount(0);
  await expectNoMorePolls(page, api);
});

test("polling stops when you leave the page", async ({ page }) => {
  const api = await openWaiting(page, job("running", ["running", "pending", "pending", "pending"]));
  await twoMorePolls(api);
  await page.getByRole("link", { name: "Warden" }).click();
  await expect(page.getByRole("heading", { name: "Paste a YouTube link." })).toBeVisible();
  await expectNoMorePolls(page, api);
});

test("F does nothing while the note is still being made", async ({ page }) => {
  const api = await openWaiting(page, job("running", ["running", "pending", "pending", "pending"]));
  await page.keyboard.press("f");
  api.answer = FINISHED;
  await expect(page.getByRole("heading", { name: "History: Robert Hooke" })).toBeVisible();
  await expect(page.locator(".page")).not.toHaveClass(/focus/);
});
