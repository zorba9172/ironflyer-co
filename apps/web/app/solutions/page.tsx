import type { Metadata } from 'next';
import { SolutionsPage } from '../marketing';

export const metadata: Metadata = {
  title: 'Solutions — Ironflyer',
  description:
    'Four shapes of work Ironflyer ships best: startup MVPs, internal tools, agency client work, and self-hosted internal AI platforms.',
  openGraph: {
    title: 'Solutions — Ironflyer',
    description: 'MVPs, internal tools, client work, and self-hosted AI platforms.',
  },
};

export default function Solutions() {
  return <SolutionsPage />;
}
