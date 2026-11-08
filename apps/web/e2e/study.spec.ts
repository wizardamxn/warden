import { expect, test, type Page } from "@playwright/test";
import { FAKE_API, fakePlayer, fixture, guarded, job, logOut, mockMe, VIDEO_ID } from "./mock";

const OTHER_ID = "aB3dE5fG7hJ";

async function mockYouTube(page: Page) {
  await page.route("https://www.youtube.com/iframe_api", (route) => route.fulfill({ contentType: "text/javascript", body: FAKE_API }));
}

async function openNote(page: Page, note: object) {
  await mockMe(page);
  await mockYouTube(page);
  await page.route(`**/v1/notes/${VIDEO_ID}`, (route) => guarded(route, job("done", ["done", "done", "done", "done"], note)));
  await page.goto(`/notes/${VIDEO_ID}`);
  await page.waitForFunction("window.fakePlayer");
}

// Fills one bubble on the answer sheet: question and option count from 0.
const fill = (page: Page, question: number, option: number) => page.locator(".q").nth(question).locator(".opt").nth(option).click();

test.describe("library", () => {
  const notes = [
    { video_id: OTHER_ID, title: "", channel: "", duration_seconds: 0, template: "neet", added_at: "2026-11-08T09:00:00+05:30", status: "queued" },
    { video_id: "zZ9yY8xX7wV", title: "Alkenes and their reactions", channel: "Chem with Ravi", duration_seconds: 3725, template: "general", added_at: "2026-11-06T09:00:00+05:30", status: "partial" },
    { video_id: VIDEO_ID, title: "Cell: the unit of life", channel: "Biology class", duration_seconds: 4119, template: "neet", added_at: "2026-11-01T09:00:00+05:30", status: "done" },
  ];

  test("lists past notes in the API's order, each linked, with status and template said quietly", async ({ page }) => {
    await mockMe(page);
    await page.route("**/v1/library", (route) => guarded(route, { notes }));
    await page.goto("/library");

    const rows = page.locator(".shelf li");
    await expect(rows).toHaveCount(3);
    await expect(rows.nth(0)).toContainText("Untitled lecture");
    await expect(rows.nth(0)).toContainText("In the queue");
    await expect(rows.nth(1)).toContainText("Chem with Ravi, 1:02:05 · Flashcards or quiz missing · General template");
    await expect(rows.nth(2)).toContainText("1 Nov");
    await expect(rows.nth(2)).toContainText("Biology class, 1:08:39");
    await expect(rows.nth(2)).not.toContainText("template");
    await expect(page.getByRole("link", { name: "Cell: the unit of life" })).toHaveAttribute("href", `/notes/${VIDEO_ID}`);
    await expect(page.getByRole("link", { name: "Library" })).toHaveAttribute("aria-current", "page");
    // Each row shows the lecture's still, as a second way in that keyboards and screen readers skip.
    await expect(rows.nth(2).locator("img")).toHaveAttribute("src", `https://i.ytimg.com/vi/${VIDEO_ID}/mqdefault.jpg`);
    await expect(rows.nth(2).locator(".still")).toHaveAttribute("tabindex", "-1");
  });

  test("the home page shows what's waiting in Revise and the last 3 lectures", async ({ page }) => {
    await mockMe(page);
    await page.route("**/v1/library", (route) => guarded(route, { notes: [...notes, { ...notes[2], video_id: "qQ1wW2eE3rR", title: "Older lecture" }] }));
    await page.route("**/v1/revise", (route) =>
      guarded(route, { cards: [VIDEO_ID, VIDEO_ID, OTHER_ID].map((video_id) => ({ video_id, title: "", template: "neet", added_at: notes[2].added_at, question: "Q", answer: "A", start_seconds: 0 })) }),
    );
    await page.goto("/");

    await expect(page.getByText("3 cards are waiting in Revise, from 2 lectures you added more than 2 days ago.")).toBeVisible();
    await expect(page.getByRole("link", { name: "Start revising" })).toHaveAttribute("href", "/revise");
    await expect(page.getByRole("heading", { name: "Your last lectures" })).toBeVisible();
    await expect(page.locator(".shelf li")).toHaveCount(3);
    await expect(page.getByRole("link", { name: "Older lecture" })).toHaveCount(0);
    await expect(page.getByRole("link", { name: "All of your library" })).toHaveAttribute("href", "/library");
  });

  test("a home page with nothing yet is just the paste box", async ({ page }) => {
    await mockMe(page);
    await page.route("**/v1/library", (route) => guarded(route, { notes: [] }));
    await page.route("**/v1/revise", (route) => guarded(route, { cards: [] }));
    await page.goto("/");
    await expect(page.getByText("1 of 3 notes today")).toBeVisible();
    await expect(page.locator(".desk")).toHaveCount(0);
  });

  test("an empty library invites pasting a link", async ({ page }) => {
    await mockMe(page);
    await page.route("**/v1/library", (route) => guarded(route, { notes: [] }));
    await page.goto("/library");
    await expect(page.getByText("Your notes will be listed here.")).toBeVisible();
    await expect(page.getByRole("link", { name: "Paste a YouTube link to make your first" })).toHaveAttribute("href", "/");
  });
});

test.describe("revise", () => {
  const cards = [
    { video_id: VIDEO_ID, title: "Cell: the unit of life", template: "neet", added_at: "2026-11-01T09:00:00+05:30", question: "Who coined the term cell?", answer: "Robert Hooke, in 1665.", start_seconds: 2645 },
    { video_id: OTHER_ID, title: "Alkenes and their reactions", template: "general", added_at: "2026-11-04T09:00:00+05:30", question: "What bond does an alkene have?", answer: "A carbon-carbon double bond.", start_seconds: 340 },
  ];

  test("shows a pile of cards, each saying its lecture, worked with the keys", async ({ page }) => {
    await mockMe(page);
    await page.route("**/v1/revise", (route) => guarded(route, { cards }));
    await page.goto("/revise");

    await expect(page.getByText("2 cards waiting from 2 lectures you added more than 2 days ago.")).toBeVisible();
    await expect(page.locator(".pile")).toHaveAttribute("data-left", "1");
    await expect(page.getByText("Who coined the term cell?")).toBeVisible();
    await expect(page.getByText("From Cell: the unit of life, added 1 Nov.")).toBeVisible();
    await expect(page.getByRole("link", { name: "Watch 44:05" })).toHaveAttribute("href", `/notes/${VIDEO_ID}?t=2645`);

    await page.keyboard.press("Space");
    await expect(page.locator(".card")).toHaveClass(/turned/);
    await expect(page.getByText("Answer: Robert Hooke, in 1665.")).toBeAttached();

    await page.keyboard.press("ArrowRight");
    await expect(page.getByText("2 of 2")).toBeVisible();
    await expect(page.locator(".card")).not.toHaveClass(/turned/);
    await expect(page.locator(".pile")).toHaveAttribute("data-left", "0");
    await expect(page.getByText("From Alkenes and their reactions, added 4 Nov.")).toBeVisible();
    await expect(page.getByText("That's the whole pile.")).toBeVisible();
  });

  test("Watch opens the lecture's note at that moment", async ({ page }) => {
    await mockMe(page);
    await mockYouTube(page);
    await page.route("**/v1/revise", (route) => guarded(route, { cards }));
    await page.route(`**/v1/notes/${VIDEO_ID}`, (route) => guarded(route, job("done", ["done", "done", "done", "done"], fixture)));
    await page.goto("/revise");
    await page.getByRole("link", { name: "Watch 44:05" }).click();

    await expect(page).toHaveURL(`/notes/${VIDEO_ID}?t=2645`);
    await page.waitForFunction("window.fakePlayer?.seeks.length");
    expect(await (await fakePlayer(page)).evaluate((player) => player.seeks)).toEqual([2645]);
    await expect(page.locator(".sec", { hasText: "History: Robert Hooke" })).toHaveAttribute("aria-current", "true");
  });

  test("with nothing waiting, it says when cards arrive", async ({ page }) => {
    await mockMe(page);
    await page.route("**/v1/revise", (route) => guarded(route, { cards: [] }));
    await page.goto("/revise");
    await expect(page.getByText("Nothing to revise yet. A lecture's flashcards arrive here 2 days after you add it")).toBeVisible();
    await expect(page.locator(".card")).toHaveCount(0);
  });

  test("with a lecture still too new, it says when its cards arrive", async ({ page }) => {
    await mockMe(page);
    await page.route("**/v1/revise", (route) => guarded(route, { cards: [] }));
    const added = new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString();
    const lecture = { video_id: OTHER_ID, title: "Alkenes and their reactions", channel: "", duration_seconds: 0, template: "neet", added_at: added, status: "done" };
    await page.route("**/v1/library", (route) => guarded(route, { notes: [lecture, { ...lecture, video_id: VIDEO_ID, title: "Failed one", status: "failed", added_at: new Date().toISOString() }] }));
    await page.goto("/revise");
    const arrives = new Date(Date.now() + 24 * 60 * 60 * 1000).toLocaleDateString("en-GB", { day: "numeric", month: "short" });
    await expect(page.getByText(`The next ones, from Alkenes and their reactions, arrive on ${arrives}`)).toBeVisible();
    await expect(page.getByRole("link", { name: "Paste a YouTube link" })).toHaveCount(0);
  });
});

test.describe("sample", () => {
  test("opens without a login and offers signing up", async ({ page }) => {
    await mockMe(page);
    await logOut(page);
    await mockYouTube(page);
    let notesCalls = 0;
    await page.route("**/v1/notes/**", (route) => {
      notesCalls++;
      return route.abort();
    });
    await page.route("**/v1/sample", (route) => route.fulfill({ json: job("done", ["done", "done", "done", "done"], fixture) }));
    await page.goto("/sample");

    await expect(page.getByRole("heading", { name: "History: Robert Hooke" })).toBeVisible();
    await expect(page).toHaveURL("/sample");
    await expect(page.getByText("A sample lecture. Sign up to make notes from your own.")).toBeVisible();
    await expect(page.getByRole("navigation").getByRole("link", { name: "Log in" })).toHaveAttribute("href", "/login?next=%2Fsample");
    await page.getByRole("button", { name: "Flashcards", exact: true }).click();
    await expect(page.getByText("1 of 11")).toBeVisible();
    expect(notesCalls).toBe(0);
  });

  test("says so when there's no sample", async ({ page }) => {
    await mockMe(page);
    await logOut(page);
    await page.route("**/v1/sample", (route) => route.fulfill({ status: 404, json: { error: "There's no sample yet." } }));
    await page.goto("/sample");
    await expect(page.getByRole("heading", { name: "There's no sample lecture yet." })).toBeVisible();
  });
});

test.describe("answer sheet scoring", () => {
  test("a NEET note is marked +4, −1 and 0", async ({ page }) => {
    await openNote(page, { ...fixture, template: "neet" });
    await page.getByRole("button", { name: "Answer sheet" }).click();
    await expect(page.getByText("+4 for right, −1 for wrong, 0 if you leave it blank.")).toBeVisible();

    await fill(page, 0, 2);
    await fill(page, 1, 1);
    await page.getByRole("button", { name: "Submit the sheet" }).click();
    await expect(page.getByRole("heading", { name: "3 out of 24" })).toBeVisible();
    await expect(page.getByText("1 right, 1 wrong, 4 left blank.")).toBeVisible();
    await expect(page.locator(".mark").nth(0)).toHaveText("+4 right");
    await expect(page.locator(".mark").nth(1)).toHaveText("−1 wrong");
  });

  test("a General note is a plain score, and nothing says NEET", async ({ page }) => {
    const general: Record<string, unknown> = {
      ...fixture,
      template: "general",
      title: "Cell: the unit of life",
      channel: "Biology class",
      quiz: fixture.quiz.filter(({ type }) => type === "direct" || type === "statement"),
    };
    delete general.ncert_chapter;
    await openNote(page, general);
    await expect(page.getByText("NCERT chapter")).toHaveCount(0);
    await page.getByRole("button", { name: "Answer sheet" }).click();
    await expect(page.getByText("Fill one bubble for each question. Your score is how many you get right.")).toBeVisible();

    await fill(page, 0, 2);
    await fill(page, 1, 1);
    await page.getByRole("button", { name: "Submit the sheet" }).click();
    await expect(page.getByRole("heading", { name: "1 out of 4" })).toBeVisible();
    await expect(page.getByText("1 right, 1 wrong, 2 left blank.")).toBeVisible();
    await expect(page.locator(".mark").nth(0)).toHaveText("✓ right");
    await expect(page.locator(".mark").nth(1)).toHaveText("✗ wrong");

    for (const view of ["Notes", "Flashcards", "Answer sheet"]) {
      await page.getByRole("button", { name: view, exact: true }).click();
      const words = await page.locator("body").innerText();
      // The lecture's own words may name NCERT; Warden's own words must not name NEET or its marking.
      expect(words).not.toMatch(/NEET|NCERT chapter|\+4|−1/);
    }
  });
});
