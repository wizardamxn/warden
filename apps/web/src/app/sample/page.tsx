import type { Metadata } from "next";
import { NoteScreen } from "@/components/notes/NoteScreen";
import { Arrive } from "@/components/ui/Arrive";

export const metadata: Metadata = { title: "A sample lecture · Warden" };

// Open to anyone, before signing up.
export default function SamplePage() {
  return (
    <Arrive>
      <NoteScreen />
    </Arrive>
  );
}
