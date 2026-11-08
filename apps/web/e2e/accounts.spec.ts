import { expect, test, type Page } from "@playwright/test";
import { guarded, logOut, mockMe, SESSION, USER, VIDEO_ID } from "./mock";

const NEW_COOKIE = `${SESSION}=fresh; HttpOnly; SameSite=Lax; Path=/; Max-Age=2592000`;
const GONE_COOKIE = `${SESSION}=; HttpOnly; SameSite=Lax; Path=/; Max-Age=0`;

// The API's signup and login: the user, plus the HttpOnly session cookie. Records what was sent.
async function mockAuth(page: Page) {
  const sent: { path: string; body: unknown }[] = [];
  for (const path of ["signup", "login"]) {
    await page.route(`**/v1/auth/${path}`, (route) => {
      sent.push({ path, body: route.request().postDataJSON() });
      return route.fulfill({ status: path === "signup" ? 201 : 200, headers: { "set-cookie": NEW_COOKIE }, json: { user: USER } });
    });
  }
  await page.route("**/v1/auth/logout", (route) => {
    sent.push({ path: "logout", body: null });
    return route.fulfill({ status: 204, headers: { "set-cookie": GONE_COOKIE } });
  });
  return sent;
}

const session = async (page: Page) => (await page.context().cookies()).find(({ name }) => name === SESSION);

test("signing up sends the template, keeps the session in an HttpOnly cookie, and lands logged in", async ({ page }) => {
  await mockMe(page);
  await logOut(page);
  const sent = await mockAuth(page);
  await page.goto("/signup");

  await expect(page.getByText("NEET lectures. Notes name the NCERT chapter")).toBeVisible();
  await expect(page.getByText("Any subject or exam.")).toBeVisible();
  await page.getByLabel("Email").fill("asha@example.com");
  await page.getByLabel("Password").fill("a long password");
  await page.locator(".choice", { hasText: "General" }).click();
  await page.getByRole("button", { name: "Sign up" }).click();

  await expect(page).toHaveURL("/");
  await expect(page.getByText("1 of 3 notes today")).toBeVisible();
  expect(sent).toEqual([{ path: "signup", body: { email: "asha@example.com", password: "a long password", template: "general" } }]);
  expect(await session(page)).toMatchObject({ value: "fresh", httpOnly: true });
  expect(await page.evaluate(() => [localStorage.length, sessionStorage.length, document.cookie])).toEqual([0, 0, ""]);
});

test("signup errors are tied to their fields", async ({ page }) => {
  await mockMe(page);
  await logOut(page);
  await page.route("**/v1/auth/signup", (route) => route.fulfill({ status: 409, json: { error: "That email already has an account." } }));
  await page.goto("/signup");

  await page.getByRole("button", { name: "Sign up" }).click();
  const email = page.getByLabel("Email");
  await expect(email).toHaveAttribute("aria-invalid", "true");
  await expect(email).toHaveAccessibleDescription("Enter your email address.");
  await expect(email).toBeFocused();
  await expect(page.getByText("Pick a template.")).toBeVisible();

  await email.fill("asha@example.com");
  await page.getByLabel("Password").fill("short");
  await page.getByRole("button", { name: "Sign up" }).click();
  await expect(page.getByLabel("Password")).toHaveAccessibleDescription("8 to 128 characters. Use at least 8 characters.");
  await expect(page.getByLabel("Password")).toBeFocused();

  await page.getByLabel("Password").fill("a long password");
  await page.locator(".choice", { hasText: "NEET" }).click();
  await page.getByRole("button", { name: "Sign up" }).click();
  await expect(email).toHaveAccessibleDescription("That email already has an account.");
  await expect(page).toHaveURL("/signup");
});

test("a wrong password shows the API's sentence, and too many tries says so", async ({ page }) => {
  await mockMe(page);
  await logOut(page);
  let answer = { status: 401, json: { error: "That email and password don't match." } };
  await page.route("**/v1/auth/login", (route) => route.fulfill(answer));
  await page.goto("/login");

  await page.getByLabel("Email").fill("asha@example.com");
  await page.getByLabel("Password").fill("wrong password");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByLabel("Password")).toHaveAccessibleDescription("That email and password don't match.");
  await expect(page.getByLabel("Email")).toHaveAttribute("aria-invalid", "true");

  answer = { status: 429, json: { error: "Too many tries. Wait a few minutes." } };
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByText("Too many tries. Wait a few minutes.")).toBeVisible();
  await expect(page).toHaveURL("/login");
});

test("a page that needs a login sends you to login, then back where you were going", async ({ page }) => {
  await mockMe(page);
  await logOut(page);
  const sent = await mockAuth(page);
  await page.route("**/v1/library", (route) => guarded(route, { notes: [] }));
  await page.goto("/library");

  await expect(page).toHaveURL("/login?next=%2Flibrary");
  await page.getByLabel("Email").fill("asha@example.com");
  await page.getByLabel("Password").fill("a long password");
  await page.getByRole("button", { name: "Log in" }).click();

  await expect(page).toHaveURL("/library");
  await expect(page.getByText("Your notes will be listed here.")).toBeVisible();
  expect(sent[0]).toEqual({ path: "login", body: { email: "asha@example.com", password: "a long password" } });
});

test("a note page needs a login too, and comes back to the same moment", async ({ page }) => {
  await mockMe(page);
  await logOut(page);
  await mockAuth(page);
  await page.route(`**/v1/notes/${VIDEO_ID}`, (route) => guarded(route, { video_id: VIDEO_ID, status: "queued", steps: [], note: null, error: "" }));
  await page.goto(`/notes/${VIDEO_ID}?t=340`);

  await expect(page).toHaveURL(`/login?next=${encodeURIComponent(`/notes/${VIDEO_ID}?t=340`)}`);
  await page.getByLabel("Email").fill("asha@example.com");
  await page.getByLabel("Password").fill("a long password");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page).toHaveURL(`/notes/${VIDEO_ID}?t=340`);
  await expect(page.getByRole("heading", { name: "In the queue." })).toBeVisible();
});

test("login only sends you back to this site", async ({ page }) => {
  await mockMe(page);
  await logOut(page);
  await mockAuth(page);
  await page.goto("/login?next=%2F%2Fevil.example");
  await page.getByLabel("Email").fill("asha@example.com");
  await page.getByLabel("Password").fill("a long password");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page).toHaveURL("/");
});

test("logging out from settings clears the session and shows the logged-out home", async ({ page }) => {
  await mockMe(page);
  const sent = await mockAuth(page);
  await page.goto("/settings");
  await expect(page.getByText(`Logged in as ${USER.email}.`)).toBeVisible();

  await page.getByRole("button", { name: "Log out" }).click();
  await expect(page).toHaveURL("/");
  await expect(page.getByRole("link", { name: "Sign up" })).toBeVisible();
  expect(sent.map(({ path }) => path)).toEqual(["logout"]);
  expect(await session(page)).toBeUndefined();
});

test("settings switches the template for new notes", async ({ page }) => {
  const me = await mockMe(page);
  let patched: unknown;
  await page.route("**/v1/me", (route) => {
    if (route.request().method() !== "PATCH") return route.fallback();
    patched = route.request().postDataJSON();
    me.user = { ...USER, template: "general" };
    return route.fulfill({ json: { user: me.user } });
  });
  await page.goto("/settings");

  await expect(page.getByRole("radio", { name: /^NEET/ })).toBeChecked();
  await expect(page.getByText("Switching only changes notes you make from now on.")).toBeVisible();
  await expect(page.getByRole("button", { name: "Save template" })).toBeDisabled();
  await page.locator(".choice", { hasText: "General" }).click();
  await page.getByRole("button", { name: "Save template" }).click();

  await expect(page.getByText("Saved. Notes you make from now on use the General template.")).toBeVisible();
  expect(patched).toEqual({ template: "general" });
  await page.reload();
  await expect(page.getByRole("radio", { name: /^General/ })).toBeChecked();
});

test("settings picks a theme for this browser, kept across visits", async ({ page }) => {
  await mockMe(page);
  await page.goto("/settings");
  const theme = () => page.evaluate(() => document.documentElement.dataset.theme ?? "device");

  await expect(page.getByRole("radio", { name: /^Same as this device/ })).toBeChecked();
  await page.locator(".choice", { hasText: "The same desk at night." }).click();
  expect(await theme()).toBe("dark");
  await page.reload();
  // Set by the script in <head>, before the page draws.
  expect(await theme()).toBe("dark");
  await expect(page.getByRole("radio", { name: /^Dark/ })).toBeChecked();

  await page.locator(".choice", { hasText: "Same as this device" }).click();
  expect(await theme()).toBe("device");
  await page.reload();
  expect(await theme()).toBe("device");
});

test("the logged-out home says what Warden does and offers the sample", async ({ page }) => {
  await mockMe(page);
  await logOut(page);
  await page.route("**/v1/sample", (route) => route.fulfill({ json: { video_id: VIDEO_ID, status: "done", steps: [], note: null, error: "" } }));
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Notes beside the video, then test yourself." })).toBeVisible();
  await expect(page.getByRole("link", { name: "Open a sample lecture" })).toHaveAttribute("href", "/sample");
  await expect(page.getByRole("link", { name: "Sign up" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Log in" })).toBeVisible();
  await expect(page.getByLabel("YouTube link")).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Library" })).toHaveCount(0);
});

test("with no sample loaded, the logged-out home leaves the link out", async ({ page }) => {
  await mockMe(page);
  await logOut(page);
  await page.route("**/v1/sample", (route) => route.fulfill({ status: 404, json: { error: "There's no sample lecture yet." } }));
  await page.goto("/");
  await expect(page.getByRole("link", { name: "Sign up" })).toBeVisible();
  await expect(page.getByText("sample")).toHaveCount(0);
});

test("a login form that isn't sent as JSON says the API's sentence", async ({ page }) => {
  await mockMe(page);
  await logOut(page);
  await page.route("**/v1/auth/login", (route) => route.fulfill({ status: 400, json: { error: "That request couldn't be read." } }));
  await page.goto("/login");
  await page.getByLabel("Email").fill("asha@example.com");
  await page.getByLabel("Password").fill("a long password");
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page.getByLabel("Email")).toHaveAccessibleDescription("That request couldn't be read.");
});

test("a settings save after the session ended goes to login", async ({ page }) => {
  await mockMe(page);
  await page.route("**/v1/me", (route) =>
    route.request().method() === "PATCH" ? route.fulfill({ status: 401, json: { error: "Log in to continue." } }) : route.fallback(),
  );
  await page.goto("/settings");
  await page.locator(".choice", { hasText: "General" }).click();
  await page.getByRole("button", { name: "Save template" }).click();
  await expect(page).toHaveURL("/login?next=%2Fsettings");
});

test("the logged-in home counts today's notes and has the quiet nav", async ({ page }) => {
  await mockMe(page);
  await page.goto("/");
  await expect(page.getByText("1 of 3 notes today")).toBeVisible();
  await expect(page.getByLabel("YouTube link")).toBeVisible();
  for (const name of ["Warden", "Library", "Revise", "Settings"]) await expect(page.getByRole("navigation").getByRole("link", { name })).toBeVisible();
});

test("at the daily limit the home says it resets tomorrow, with no paste box", async ({ page }) => {
  await mockMe(page, { ...USER, notes_today: 3 });
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "That's your 3 notes for today." })).toBeVisible();
  await expect(page.getByText("You can make more tomorrow")).toBeVisible();
  await expect(page.getByLabel("YouTube link")).toHaveCount(0);
});

for (const [status, error] of [
  [429, "You've made 3 notes today. More tomorrow."],
  [503, "Warden is at its limit for today. Try again tomorrow."],
] as const) {
  test(`a ${status} from the API while pasting is said plainly`, async ({ page }) => {
    await mockMe(page, { ...USER, notes_today: 2 });
    await page.route("**/v1/notes", (route) => route.fulfill({ status, json: { error } }));
    await page.goto("/");
    await page.getByLabel("YouTube link").fill(`https://youtu.be/${VIDEO_ID}`);
    await page.getByRole("button", { name: "Make notes" }).click();
    await expect(page.getByText(error)).toBeVisible();
    await expect(page.getByLabel("YouTube link")).toHaveAccessibleDescription(error);
    await expect(page).toHaveURL("/");
  });
}
