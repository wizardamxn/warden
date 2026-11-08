const API = process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8080";
// A server that takes the connection and then hangs must not leave the page waiting forever.
const TIMEOUT_MS = 10_000;
export const DOWN = "Warden's server isn't answering. Give it a minute.";

// The API's own plain sentence, with the status so pages can tell "log in first" (401) from the rest.
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message);
  }
}

export const needsLogin = (problem: unknown) => problem instanceof ApiError && problem.status === 401;

// Every call sends the session cookie. The session lives only in the API's HttpOnly cookie, never in this website's storage.
export async function call(path: string, init: RequestInit = {}): Promise<Response> {
  const timeout = AbortSignal.timeout(TIMEOUT_MS);
  try {
    return await fetch(`${API}${path}`, {
      ...init,
      credentials: "include",
      signal: init.signal ? AbortSignal.any([init.signal, timeout]) : timeout,
    });
  } catch (problem) {
    if (init.signal?.aborted) throw problem;
    throw new Error(DOWN);
  }
}

export function send(path: string, method: string, body?: object): Promise<Response> {
  return call(path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
}

// The body of a good answer. Anything else throws the API's sentence.
export async function read<T>(response: Response): Promise<T> {
  const body = await response.json().catch(() => null);
  if (!response.ok || !body) throw new ApiError(body?.error || DOWN, response.status);
  return body;
}
