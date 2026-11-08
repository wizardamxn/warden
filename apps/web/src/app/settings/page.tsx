import type { Metadata } from "next";
import { Settings } from "@/components/auth/Settings";
import { Arrive } from "@/components/ui/Arrive";

export const metadata: Metadata = { title: "Settings · Warden" };

export default function SettingsPage() {
  return (
    <Arrive>
      <Settings />
    </Arrive>
  );
}
