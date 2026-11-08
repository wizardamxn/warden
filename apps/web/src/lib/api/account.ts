import { call, read, send } from "./client";

export type Template = "neet" | "general";

// `notes_today` counts lectures added today, India time. Try again and lectures already in the library don't count.
export type User = {
  id: number;
  email: string;
  template: Template;
  notes_today: number;
  daily_limit: number;
};

// Signup, login and both /v1/me routes answer `{"user": {...}}`.
async function readUser(response: Response): Promise<User> {
  return (await read<{ user: User }>(response)).user;
}

export async function signup(email: string, password: string, template: Template): Promise<User> {
  return readUser(await send("/v1/auth/signup", "POST", { email, password, template }));
}

export async function login(email: string, password: string): Promise<User> {
  return readUser(await send("/v1/auth/login", "POST", { email, password }));
}

export async function logout(): Promise<void> {
  const response = await send("/v1/auth/logout", "POST");
  if (!response.ok) await read(response);
}

// Null when nobody is logged in.
export async function getMe(): Promise<User | null> {
  const response = await call("/v1/me");
  return response.status === 401 ? null : readUser(response);
}

// Only notes made after the change use the new template.
export async function setTemplate(template: Template): Promise<User> {
  return readUser(await send("/v1/me", "PATCH", { template }));
}
