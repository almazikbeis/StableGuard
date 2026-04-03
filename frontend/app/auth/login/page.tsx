import { redirect } from "next/navigation";

// Email/password auth removed — Web3 wallet auth is the only login method.
export default function LoginPage() {
  redirect("/onboarding");
}
