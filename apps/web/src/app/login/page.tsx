import type { Metadata } from "next";
import { AuthForm } from "@/components/auth/AuthForm";
import { Arrive } from "@/components/ui/Arrive";

export const metadata: Metadata = { title: "Log in to Warden" };

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ next?: string }> }) {
  return (
    <Arrive>
      <AuthForm mode="login" next={(await searchParams).next} />
    </Arrive>
  );
}
