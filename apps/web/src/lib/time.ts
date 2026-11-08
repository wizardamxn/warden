// mm:ss, or h:mm:ss from one hour up.
export function formatTime(totalSeconds: number): string {
  const total = Math.max(0, Math.floor(totalSeconds));
  const hours = Math.floor(total / 3600);
  const minutes = String(Math.floor((total % 3600) / 60)).padStart(2, "0");
  const seconds = String(total % 60).padStart(2, "0");
  return hours ? `${hours}:${minutes}:${seconds}` : `${minutes}:${seconds}`;
}

// "3 Nov", with the year when it isn't this year.
export function formatDay(iso: string): string {
  const day = new Date(iso);
  const thisYear = day.getFullYear() === new Date().getFullYear();
  return day.toLocaleDateString("en-GB", { day: "numeric", month: "short", year: thisYear ? undefined : "numeric" });
}
