import type { Template } from "./account";
import { call, read, send } from "./client";

// The parts of a note this website shows. Older notes may lack the optional fields,
// and Go writes an empty list as null, so lists are read with `?? []`.
export type Note = {
  // Older notes were all made before templates, with the NEET one.
  template?: Template;
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

// One past note in the student's library, newest first. `title` is "" until the captions step is done.
export type LibraryNote = {
  video_id: string;
  title: string;
  channel: string;
  duration_seconds: number;
  template: Template;
  added_at: string;
  status: NoteJob["status"];
};

// A flashcard waiting in Revise, with the lecture it came from.
export type ReviseCard = Flashcard & {
  video_id: string;
  title: string;
  template: Template;
  added_at: string;
};

// `note` appears once the notes step is done. `partial`: the note exists but flashcards or quiz failed. `failed`: no note exists.
export type NoteJob = {
  video_id: string;
  status: "queued" | "running" | "done" | "partial" | "failed";
  steps: Step[];
  note: Note | null;
  error: string;
};

// Nothing more will happen without a retry, so polling can stop.
export function isFinished(job: NoteJob): boolean {
  return ["done", "partial", "failed"].includes(job.status);
}

async function readJob(response: Response): Promise<NoteJob> {
  const body = await read<NoteJob>(response);
  return { ...body, steps: body.steps ?? [] };
}

// Starts the note if it doesn't exist yet, and returns straight away.
export async function makeNote(url: string): Promise<NoteJob> {
  return readJob(await send("/v1/notes", "POST", { url }));
}

// Returns null when no note exists for this video in the student's library.
export async function getNote(videoId: string, signal: AbortSignal): Promise<NoteJob | null> {
  const response = await call(`/v1/notes/${videoId}`, { signal });
  return response.status === 404 ? null : readJob(response);
}

// The finished sample lecture, open without a login. Null when none is set up.
export async function getSample(signal: AbortSignal): Promise<NoteJob | null> {
  const response = await call("/v1/sample", { signal });
  return response.status === 404 ? null : readJob(response);
}

// Re-queues only the failed steps. Returns null when no note exists for this video.
export async function retryNote(videoId: string): Promise<NoteJob | null> {
  const response = await send(`/v1/notes/${videoId}/retry`, "POST");
  return response.status === 404 ? null : readJob(response);
}

export async function getLibrary(): Promise<LibraryNote[]> {
  return (await read<{ notes: LibraryNote[] }>(await call("/v1/library"))).notes;
}

export async function getRevise(): Promise<ReviseCard[]> {
  return (await read<{ cards: ReviseCard[] }>(await call("/v1/revise"))).cards;
}
