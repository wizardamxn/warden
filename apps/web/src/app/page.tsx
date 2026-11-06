import { PasteLink } from "@/components/notes/PasteLink";

export default function Home() {
  return (
    <div className="page">
      <div className="glow" aria-hidden="true" />
      <main className="wrap">
        <div className="nav">
          <span className="brand">Warden</span>
        </div>
        <div className="home">
          <h1>Paste a YouTube link.</h1>
          <p className="lead">Get notes beside the video, with every section tied to its moment in the lecture.</p>
          <PasteLink />
        </div>
      </main>
    </div>
  );
}
