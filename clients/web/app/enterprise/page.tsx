import type { Metadata } from "next";
import { Suspense } from "react";
import { Base44PublicPage } from "../../src/components/marketing/Base44PublicPage";

export const metadata: Metadata = {
  title: "Enterprise - IronFlyer",
  description:
    "SSO, audit logs, private deployment, RBAC and code ownership for teams using IronFlyer.",
};

export default function EnterprisePage() {
  return (
    <Suspense fallback={null}>
      <Base44PublicPage page="enterprise" />
    </Suspense>
  );
}
