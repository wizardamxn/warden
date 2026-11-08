import type { Metadata } from "next";
import { Revise } from "@/components/study/Revise";
import { Arrive } from "@/components/ui/Arrive";

export const metadata: Metadata = { title: "Revise · Warden" };

export default function RevisePage() {
  return (
    <Arrive>
      <Revise />
    </Arrive>
  );
}
