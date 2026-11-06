import { useCallback, useEffect, useRef, useState, type RefObject } from "react";

// The small part of the IFrame Player API we use.
type Player = {
  getCurrentTime(): number;
  seekTo(seconds: number, allowSeekAhead: boolean): void;
  playVideo(): void;
  destroy(): void;
};

type PlayerOptions = {
  host: string;
  videoId: string;
  width: string;
  height: string;
  playerVars: Record<string, string | number>;
  events: { onReady(): void; onError(event: { data: number }): void };
};

declare global {
  interface Window {
    YT?: { Player: new (element: HTMLElement, options: PlayerOptions) => Player };
    onYouTubeIframeAPIReady?: () => void;
  }
}

// 101 and 150: the owner blocked embedding. 153: YouTube refused this page as the embedder.
const CANT_EMBED = [101, 150, 153];
const CHECK_EVERY_MS = 500;

let api: Promise<void> | undefined;

function loadApi(): Promise<void> {
  api ??= new Promise((resolve, reject) => {
    if (window.YT?.Player) return resolve();
    window.onYouTubeIframeAPIReady = resolve;
    const script = document.createElement("script");
    script.src = "https://www.youtube.com/iframe_api";
    script.onerror = () => {
      api = undefined;
      reject();
    };
    document.head.append(script);
  });
  return api;
}

// Puts the YouTube player inside `frame`, and reports its time about twice a second.
export function useYouTubePlayer(frame: RefObject<HTMLDivElement | null>, videoId: string, onTime: (seconds: number) => void) {
  const [blocked, setBlocked] = useState(false);
  const player = useRef<Player | null>(null);
  const earlySeek = useRef<number | null>(null);

  useEffect(() => {
    const parent = frame.current;
    if (!parent || blocked) return;
    let stopped = false;
    let created: Player | undefined;
    let timer: ReturnType<typeof setInterval>;

    loadApi().then(
      () => {
        if (stopped || !window.YT) return;
        // YouTube replaces this element with its iframe, so React must not own it.
        const element = parent.appendChild(document.createElement("div"));
        const yt: Player = new window.YT.Player(element, {
          host: "https://www.youtube-nocookie.com",
          videoId,
          width: "100%",
          height: "100%",
          playerVars: { origin: window.location.origin, playsinline: 1, rel: 0 },
          events: {
            onReady() {
              player.current = yt;
              if (earlySeek.current !== null) {
                yt.seekTo(earlySeek.current, true);
                yt.playVideo();
              }
              timer = setInterval(() => onTime(yt.getCurrentTime()), CHECK_EVERY_MS);
            },
            onError({ data }) {
              if (CANT_EMBED.includes(data)) setBlocked(true);
            },
          },
        });
        created = yt;
      },
      () => setBlocked(true),
    );

    return () => {
      stopped = true;
      clearInterval(timer);
      created?.destroy();
      player.current = null;
      parent.replaceChildren();
    };
  }, [frame, videoId, onTime, blocked]);

  const seek = useCallback((seconds: number) => {
    earlySeek.current = seconds;
    player.current?.seekTo(seconds, true);
    player.current?.playVideo();
  }, []);

  return { blocked, seek };
}
