import type { Metadata } from 'next';
import { SecurityPage } from '../marketing';

export const metadata: Metadata = {
  title: 'Security — Ironflyer',
  description:
    'Encryption, secret storage, tenant isolation, patch lifecycle, data retention, AI provider posture. SOC 2 in progress. DPA available.',
  openGraph: {
    title: 'Security at Ironflyer',
    description: 'Tenant isolation, patch lifecycle, encryption, SOC 2 in progress.',
  },
};

export default function Security() {
  return <SecurityPage />;
}
