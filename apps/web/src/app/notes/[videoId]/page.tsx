import { notFound } from "next/navigation";
import { NoteScreen } from "@/components/notes/NoteScreen";
import { isVideoId } from "@/lib/youtube";

export default async function NotePage({ params }: { params: Promise<{ videoId: string }> }) {
  const { videoId } = await params;
  if (!isVideoId(videoId)) notFound();
  return <NoteScreen videoId={videoId} />;
}
