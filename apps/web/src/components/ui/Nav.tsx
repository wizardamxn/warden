"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

const SITE: [string, string][] = [
  ["/library", "Library"],
  ["/revise", "Revise"],
  ["/settings", "Settings"],
];

// The quiet top line. `signedIn` true shows the student's pages, false offers login, undefined shows neither.
// `children` sit after the name (a note's views); `end` sits last (the Focus button).
export function Nav({ signedIn, children, end }: { signedIn?: boolean; children?: ReactNode; end?: ReactNode }) {
  const path = usePathname();
  const links: [string, string][] = signedIn
    ? SITE
    : signedIn === false
      ? [
          [`/login?next=${encodeURIComponent(path)}`, "Log in"],
          [`/signup?next=${encodeURIComponent(path)}`, "Sign up"],
        ]
      : [];

  return (
    // Named so it stays still while the page under it changes (globals.css).
    <nav className="nav" aria-label="Warden" style={{ viewTransitionName: "site-nav" }}>
      <Link className="brand" href="/">
        Warden
      </Link>
      {children}
      <span className="site">
        {links.map(([href, label]) => (
          <Link className="quiet" href={href} key={label} aria-current={path === href ? "page" : undefined}>
            {label}
          </Link>
        ))}
      </span>
      {end}
    </nav>
  );
}
