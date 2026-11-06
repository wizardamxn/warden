import Link from "next/link";
import "@/components/notes/notes.css";

export default function NotFound() {
  return (
    <div className="page">
      <div className="glow" aria-hidden="true" />
      <main className="wrap">
        <div className="nav">
          <Link className="brand" href="/">
            Warden
          </Link>
        </div>
        <div className="home">
          <h1>There&apos;s nothing at this address.</h1>
          <Link className="link" href="/">
            Paste a YouTube link
          </Link>
        </div>
      </main>
    </div>
  );
}
