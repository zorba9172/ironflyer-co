import type { Metadata } from 'next';
import { ProductPage } from '../marketing';

export const metadata: Metadata = {
  title: 'Product — Ironflyer Finisher Loop',
  description:
    'Nine gates: Spec, UX, Architecture, Code, Lint, Tests, Security, Budget, Deploy. See how the finisher loop turns a prompt into a production-ready product.',
  openGraph: {
    title: 'The Ironflyer Finisher Loop',
    description: 'Spec to deploy in nine gates. Real Linux sandbox. Multi-provider routing.',
  },
};

export default function Product() {
  return <ProductPage />;
}
