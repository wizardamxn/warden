import type { NextConfig } from "next";

const config: NextConfig = {
  // Keeps the page clear of Next's badge, which sits over the notes in screenshots and focus mode.
  devIndicators: false,
  // Stops `next dev` from writing an AGENTS.md here.
  agentRules: false,
};

export default config;
