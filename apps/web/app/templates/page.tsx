import type { Metadata } from 'next';
import { TemplatesPage } from '../marketing';

export const metadata: Metadata = {
  title: 'Templates — Ironflyer',
  description:
    'Twelve starter prompts that run through the finisher gates: SaaS dashboard, AI chatbot, marketplace, internal admin, client portal, e-commerce, more.',
  openGraph: {
    title: 'Ironflyer templates',
    description: 'Production-ready starter prompts for SaaS, AI, internal tools, marketplaces.',
  },
};

export default function Templates() {
  return <TemplatesPage />;
}
