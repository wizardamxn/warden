import Link from "next/link";
import { Arrive } from "@/components/ui/Arrive";
import { Nav } from "@/components/ui/Nav";
import "@/components/notes/notes.css";

export default function NotFound() {
  return (
    <Arrive>
      <div className="page">
        <div className="glow" aria-hidden="true" />
        <main className="wrap">
          <Nav />
          <div className="home">
            <h1>There&apos;s nothing at this address.</h1>
            <Link className="link" href="/">
              Paste a YouTube link
            </Link>
          </div>
        </main>
      </div>
    </Arrive>
  );
}
