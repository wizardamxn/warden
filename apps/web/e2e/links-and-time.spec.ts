import { expect, test } from "@playwright/test";
import { formatTime } from "../src/lib/time";
import { canonicalUrl, parseVideoId } from "../src/lib/youtube";

const ID = "k7Zb1iOV-ys";

test("parseVideoId takes every YouTube link shape", () => {
  for (const link of [
    `https://www.youtube.com/watch?v=${ID}`,
    `https://youtube.com/watch?list=abc&v=${ID}&t=30s`,
    `http://m.youtube.com/watch?v=${ID}`,
    `https://youtu.be/${ID}?si=abc`,
    `https://www.youtube.com/embed/${ID}`,
    `https://www.youtube.com/shorts/${ID}`,
    `https://www.youtube.com/live/${ID}`,
    `  https://www.youtube-nocookie.com/embed/${ID}  `,
  ]) {
    expect(parseVideoId(link), link).toBe(ID);
  }
});

test("parseVideoId refuses everything else", () => {
  for (const link of [
    "",
    ID,
    "https://www.youtube.com/",
    "https://www.youtube.com/playlist?list=abc",
    `https://example.com/watch?v=${ID}`,
    `https://youtube.com.evil.example/watch?v=${ID}`,
    `https://evil.example/?x=youtube.com/watch?v=${ID}`,
    `javascript:alert("https://youtu.be/${ID}")`,
    `https://youtu.be/${ID}x`,
    "https://youtu.be/k7Zb1iOV-y;",
    "https://www.youtube.com/watch?v=..%2F..%2F.env",
  ]) {
    expect(parseVideoId(link), link).toBeNull();
  }
});

test("canonicalUrl rebuilds the link from the ID alone", () => {
  expect(canonicalUrl(ID)).toBe(`https://www.youtube.com/watch?v=${ID}`);
});

test("formatTime pads minutes and shows hours only past an hour", () => {
  expect(formatTime(0)).toBe("00:00");
  expect(formatTime(456)).toBe("07:36");
  expect(formatTime(3599)).toBe("59:59");
  expect(formatTime(3600)).toBe("1:00:00");
  expect(formatTime(4119.9)).toBe("1:08:39");
});
