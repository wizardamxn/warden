// The parts of a note this website shows. Older notes may lack the optional fields,
// and Go writes an empty list as null, so lists are read with `?? []`.
export type Note = {
  title: string;
  channel: string;
  duration_seconds: number;
  ncert_chapter?: string;
  tldr: string;
  key_points: string[] | null;
  sections: Section[] | null;
  mnemonics?: { topic: string; text: string }[] | null;
  glossary: { term: string; definition: string }[] | null;
  flashcards?: Flashcard[] | null;
  quiz?: Question[] | null;
  failed_steps?: string[] | null;
};

export type Flashcard = {
  question: string;
  answer: string;
  start_seconds: number;
};

// `question` can hold several lines: statements, assertion and reason, or the two lists of a match question.
export type Question = {
  type: "direct" | "statement" | "assertion-reason" | "match";
  question: string;
  options: string[];
  answer_index: number;
  explanation: string;
  start_seconds: number;
};

export type Section = {
  start_seconds: number;
  title: string;
  bullets: string[] | null;
  exam_mark?: string;
};

export type Step = {
  name: "captions" | "notes" | "flashcards" | "quiz";
  status: "pending" | "running" | "done" | "failed";
  error: string;
};

// `note` appears once the notes step is done. `partial`: the note exists but flashcards or quiz failed. `failed`: no note exists.
export type NoteJob = {
  video_id: string;
  status: "queued" | "running" | "done" | "partial" | "failed";
  steps: Step[];
  note: Note | null;
  error: string;
};

const API = process.env.NEXT_PUBLIC_API_URL || "http://127.0.0.1:8080";
// A server that takes the connection and then hangs must not leave the page waiting forever.
const TIMEOUT_MS = 10_000;
const DOWN = "Warden's server isn't answering. Give it a minute.";

async function call(path: string, init: RequestInit = {}): Promise<Response> {
  const timeout = AbortSignal.timeout(TIMEOUT_MS);
  try {
    return await fetch(`${API}${path}`, { ...init, signal: init.signal ? AbortSignal.any([init.signal, timeout]) : timeout });
  } catch (problem) {
    if (init.signal?.aborted) throw problem;
    throw new Error(DOWN);
  }
}

async function read(response: Response): Promise<NoteJob> {
  const body = await response.json().catch(() => null);
  if (!response.ok || !body) throw new Error(body?.error || DOWN);
  return { ...body, steps: body.steps ?? [] };
}

// Nothing more will happen without a retry, so polling can stop.
export function isFinished(job: NoteJob): boolean {
  return ["done", "partial", "failed"].includes(job.status);
}

// Starts the note if it doesn't exist yet, and returns straight away.
export async function makeNote(url: string): Promise<NoteJob> {
  return read(
    await call("/v1/notes", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ url }),
    }),
  );
}

// Returns null when no note exists for this video.
export async function getNote(videoId: string, signal: AbortSignal): Promise<NoteJob | null> {
  const response = await call(`/v1/notes/${videoId}`, { signal });
  return response.status === 404 ? null : read(response);
}

// Re-queues only the failed steps. Returns null when no note exists for this video.
export async function retryNote(videoId: string): Promise<NoteJob | null> {
  const response = await call(`/v1/notes/${videoId}/retry`, { method: "POST" });
  return response.status === 404 ? null : read(response);
}
