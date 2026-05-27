import type { Metadata } from "next";
import { Suspense } from "react";
import { Base44PublicPage } from "../../src/components/marketing/Base44PublicPage";

export const metadata: Metadata = {
  title: "Pricing - IronFlyer",
  description:
    "Simple pricing for prompt-to-product work: free, pro, team and enterprise.",
};

export default function PricingPage() {
  return (
    <Suspense fallback={null}>
      <Base44PublicPage page="pricing" />
    </Suspense>
  );
}
