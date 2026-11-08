import type { Metadata } from "next";
import { AuthForm } from "@/components/auth/AuthForm";
import { Arrive } from "@/components/ui/Arrive";

export const metadata: Metadata = { title: "Sign up for Warden" };

export default async function SignupPage({ searchParams }: { searchParams: Promise<{ next?: string }> }) {
  return (
    <Arrive>
      <AuthForm mode="signup" next={(await searchParams).next} />
    </Arrive>
  );
}
