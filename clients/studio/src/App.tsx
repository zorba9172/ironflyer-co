import { lazy, Suspense } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { Box, CircularProgress } from '@mui/material';
import { AppShell } from './AppShell';

// Every route is code-split: the initial load ships the shell only, and each
// page (the Editor especially, with its heavy panes) loads on navigation.
const StudioHome = lazy(() => import('./pages/StudioHome').then((m) => ({ default: m.StudioHome })));
const ProjectsPage = lazy(() => import('./pages/ProjectsPage').then((m) => ({ default: m.ProjectsPage })));
const TemplatesPage = lazy(() => import('./pages/TemplatesPage').then((m) => ({ default: m.TemplatesPage })));
const IntegrationsPage = lazy(() => import('./pages/IntegrationsPage').then((m) => ({ default: m.IntegrationsPage })));
const AgentsPage = lazy(() => import('./pages/AgentsPage').then((m) => ({ default: m.AgentsPage })));
const PlansPage = lazy(() => import('./pages/PlansPage').then((m) => ({ default: m.PlansPage })));
const Editor = lazy(() => import('./pages/Editor').then((m) => ({ default: m.Editor })));
const Landing = lazy(() => import('./pages/Landing').then((m) => ({ default: m.Landing })));

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
        <Route path="/" element={<StudioHome />} />
        <Route element={<AppShell />}>
          <Route path="/studio" element={<ProjectsPage />} />
          <Route path="/projects" element={<ProjectsPage />} />
          <Route path="/templates" element={<TemplatesPage />} />
          <Route path="/integrations" element={<IntegrationsPage />} />
          <Route path="/agents" element={<AgentsPage />} />
          <Route path="/plans" element={<PlansPage />} />
        </Route>
        <Route path="/build" element={<Editor />} />
        {/* The Neon Intelligence landing — the logged-out marketing entry,
            reachable for preview in any auth mode. */}
        <Route path="/welcome" element={<Landing onEnter={() => undefined} />} />
        {/* Unknown paths fall back to the studio home rather than a blank screen. */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  );
}
