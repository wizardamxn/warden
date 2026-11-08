import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { getMe, type User } from "@/lib/api/account";

// Where to go after logging in. Only paths on this site, so a link can't send someone elsewhere.
// Browsers drop tabs and newlines from addresses ("/\t/evil.example" becomes "//evil.example"), so no spaces at all.
export function nextPath(next: unknown): string {
  return typeof next === "string" && /^\/(?![/\\])\S*$/.test(next) ? next : "/";
}

// Sends a logged-out visitor to login, which brings them back here afterwards.
export function useToLogin() {
  const router = useRouter();
  return useCallback(() => router.replace(`/login?next=${encodeURIComponent(location.pathname + location.search)}`), [router]);
}

// The logged-in student: undefined while asking, null when logged out. `required` pages send logged-out visitors to login.
export function useMe(required: boolean) {
  const [user, setUser] = useState<User | null>();
  const [problem, setProblem] = useState("");
  const toLogin = useToLogin();

  useEffect(() => {
    let left = false;
    getMe().then(
      (found) => {
        if (left) return;
        if (!found && required) return toLogin();
        setUser(found);
      },
      (failure: Error) => !left && setProblem(failure.message),
    );
    return () => {
      left = true;
    };
  }, [required, toLogin]);

  return { user, setUser, problem };
}
