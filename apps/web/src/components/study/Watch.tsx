import { formatTime } from "@/lib/time";
import { watchUrlAt } from "@/lib/youtube";

// "Watch 05:40": back to the moment in the video. `onWatch` is null when the video can't play here, so it opens YouTube instead.
export function Watch({ videoId, seconds, onWatch }: { videoId: string; seconds: number; onWatch: ((seconds: number) => void) | null }) {
  const label = `Watch ${formatTime(seconds)}`;
  return onWatch ? (
    <button className="watch" type="button" onClick={() => onWatch(seconds)}>
      {label}
    </button>
  ) : (
    <a className="watch" href={watchUrlAt(videoId, seconds)} target="_blank" rel="noopener noreferrer" aria-label={`${label} on YouTube in a new tab`}>
      {label}
    </a>
  );
}
