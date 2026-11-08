import type { Metadata } from "next";
import { Library } from "@/components/library/Library";
import { Arrive } from "@/components/ui/Arrive";

export const metadata: Metadata = { title: "Library · Warden" };

export default function LibraryPage() {
  return (
    <Arrive>
      <Library />
    </Arrive>
  );
}
