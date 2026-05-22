import type { Metadata } from 'next';
import { EnterprisePage } from '../marketing';

export const metadata: Metadata = {
  title: 'Enterprise — Ironflyer',
  description:
    'SSO, audit logs, on-prem Helm deploy, dedicated CSM, 99.9% SLA. SOC 2 in progress. Talk to sales for a demo and a deployment plan.',
  openGraph: {
    title: 'Ironflyer for Enterprise',
    description: 'SSO + audit + on-prem deploy. SOC 2 in progress.',
  },
};

export default function Enterprise() {
  return <EnterprisePage />;
}
