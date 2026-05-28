import type { RouteRecord } from 'vite-react-ssg';
import { RootLayout } from './RootLayout';
import { Home } from './pages/Home';
import { Product } from './pages/Product';
import { Studio } from './pages/Studio';
import { Pricing } from './pages/Pricing';
import { Manifesto } from './pages/Manifesto';

export const routes: RouteRecord[] = [
  {
    path: '/',
    element: <RootLayout />,
    children: [
      { index: true, element: <Home /> },
      { path: 'product', element: <Product /> },
      { path: 'studio', element: <Studio /> },
      { path: 'pricing', element: <Pricing /> },
      { path: 'manifesto', element: <Manifesto /> },
    ],
  },
];
