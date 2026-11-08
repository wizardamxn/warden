import type { Metadata } from "next";
import { Instrument_Sans, Newsreader } from "next/font/google";
import type { ReactNode } from "react";
import "./globals.css";

const newsreader = Newsreader({
  subsets: ["latin"],
  axes: ["opsz"],
  style: ["normal", "italic"],
  variable: "--font-newsreader",
});

const instrumentSans = Instrument_Sans({
  subsets: ["latin"],
  variable: "--font-instrument-sans",
});

export const metadata: Metadata = {
  title: "Warden",
  description: "Paste a YouTube lecture link and get notes beside the video, then test yourself.",
};

// Runs before the page paints, so a theme picked in Settings never flashes the other one first.
// Matches THEME_KEY in components/ui/ThemePicker.tsx.
const THEME = `try{var t=localStorage.getItem("warden-theme");if(t==="light"||t==="dark")document.documentElement.dataset.theme=t}catch(e){}`;

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    // The theme script changes <html> before React loads, so React mustn't complain that it differs.
    <html lang="en" className={`${newsreader.variable} ${instrumentSans.variable}`} suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: THEME }} />
      </head>
      <body>{children}</body>
    </html>
  );
}
