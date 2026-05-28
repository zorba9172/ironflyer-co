import { Routes, Route } from 'react-router-dom';
import { AppShell } from './AppShell';
import { StudioHome } from './pages/StudioHome';
import { ProjectsPage } from './pages/ProjectsPage';
import { TemplatesPage } from './pages/TemplatesPage';
import { IntegrationsPage } from './pages/IntegrationsPage';
import { AgentsPage } from './pages/AgentsPage';
import { Editor } from './pages/Editor';

export function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route path="/" element={<StudioHome />} />
        <Route path="/projects" element={<ProjectsPage />} />
        <Route path="/templates" element={<TemplatesPage />} />
        <Route path="/integrations" element={<IntegrationsPage />} />
        <Route path="/agents" element={<AgentsPage />} />
      </Route>
      <Route path="/build" element={<Editor />} />
    </Routes>
  );
}
