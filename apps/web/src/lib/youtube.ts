const VIDEO_ID = /^[A-Za-z0-9_-]{11}$/;

const WATCH_HOSTS = new Set([
  "youtube.com",
  "www.youtube.com",
  "m.youtube.com",
  "music.youtube.com",
  "www.youtube-nocookie.com",
]);

export function isVideoId(id: string): boolean {
  return VIDEO_ID.test(id);
}

// Returns null for anything that isn't a link to one YouTube video.
export function parseVideoId(link: string): string | null {
  let url: URL;
  try {
    url = new URL(link.trim());
  } catch {
    return null;
  }
  if (url.protocol !== "https:" && url.protocol !== "http:") return null;

  const [first, second] = url.pathname.split("/").filter(Boolean);
  let id: string | null | undefined;
  if (url.hostname === "youtu.be") {
    id = first;
  } else if (WATCH_HOSTS.has(url.hostname)) {
    id = first === "watch" ? url.searchParams.get("v") : ["embed", "shorts", "live", "v"].includes(first) ? second : null;
  }
  return id && isVideoId(id) ? id : null;
}

export function canonicalUrl(videoId: string): string {
  return `https://www.youtube.com/watch?v=${videoId}`;
}

export function watchUrlAt(videoId: string, seconds: number): string {
  return `${canonicalUrl(videoId)}&t=${Math.floor(seconds)}s`;
}
