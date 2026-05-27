import type { Metadata } from "next";
import { Suspense } from "react";
import { Base44PublicPage } from "../../src/components/marketing/Base44PublicPage";

export const metadata: Metadata = {
  title: "AppSec — Ironflyer",
  description:
    "Semgrep, gitleaks, trufflehog and govulncheck on every AI iteration. Critical findings block the deploy lane. SOC2 and HIPAA gates ship in the codebase. This is the production discipline Lovable, Bolt, Base44 and v0 don't sell.",
};

export default function AppSecPage() {
  return (
    <Suspense fallback={null}>
      <Base44PublicPage page="appsec" />
    </Suspense>
  );
}
