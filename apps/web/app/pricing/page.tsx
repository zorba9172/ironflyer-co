import type { Metadata } from 'next';
import { PricingPage } from '../marketing';

export const metadata: Metadata = {
  title: 'Pricing — Ironflyer',
  description:
    'Flat subscription, no credit packs, live margin. Calculate your AI cost, see what the budget gate does, compare Starter / Pro / Team / Enterprise.',
  openGraph: {
    title: 'Pricing — Ironflyer',
    description: 'Flat subscription, transparent provider cost, live margin. No credit packs.',
  },
};

export default function Pricing() {
  return <PricingPage />;
}
