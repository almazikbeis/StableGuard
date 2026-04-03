import { redirect } from "next/navigation";

// Email/password registration removed — connect wallet via onboarding instead.
export default function RegisterPage() {
  redirect("/onboarding");
}
