import { notFound } from "next/navigation";
import { NoteScreen } from "@/components/notes/NoteScreen";
import { isVideoId } from "@/lib/youtube";
import { Arrive } from "@/components/ui/Arrive";

// `?t=340` opens the note at that second, for Watch links from Revise.
export default async function NotePage({
  params,
  searchParams,
}: {
  params: Promise<{ videoId: string }>;
  searchParams: Promise<{ t?: string }>;
}) {
  const { videoId } = await params;
  if (!isVideoId(videoId)) notFound();
  const start = Number((await searchParams).t);
  return (
    <Arrive>
      <NoteScreen videoId={videoId} start={Number.isFinite(start) && start > 0 ? Math.floor(start) : 0} />
    </Arrive>
  );
}
