// app/pricing/page.tsx — public marketing route.

import { PricingPage } from "../../src/components/PricingPage";

export const metadata = {
  title: "Pricing — Ironflyer",
  description:
    "Wallet, not subscription. Pay only for finished runs against an honest provider rate sheet.",
};

export default function Page() {
  return <PricingPage />;
}
