import { lazy, Suspense } from 'react';
import { Routes, Route } from 'react-router-dom';
import { Box, CircularProgress } from '@mui/material';
import { AppShell } from './AppShell';

// Internal operator admin: revenue, projects, wallet/spend, and audit — all
// rendered inside the persistent sidebar shell. Each page is code-split so the
// shell boots first and a page's heavy deps (tables/charts) load on navigation.
const Overview = lazy(() => import('./pages/Overview').then((m) => ({ default: m.Overview })));
const Projects = lazy(() => import('./pages/Projects').then((m) => ({ default: m.Projects })));
const Wallet = lazy(() => import('./pages/Wallet').then((m) => ({ default: m.Wallet })));
const Audit = lazy(() => import('./pages/Audit').then((m) => ({ default: m.Audit })));

function RouteFallback() {
  return (
    <Box sx={{ minHeight: '60vh', display: 'grid', placeItems: 'center' }}>
      <CircularProgress size={26} thickness={5} />
    </Box>
  );
}

export function App() {
  return (
    <Suspense fallback={<RouteFallback />}>
      <Routes>
        <Route element={<AppShell />}>
          <Route path="/" element={<Overview />} />
          <Route path="/projects" element={<Projects />} />
          <Route path="/wallet" element={<Wallet />} />
          <Route path="/audit" element={<Audit />} />
        </Route>
      </Routes>
    </Suspense>
  );
}
