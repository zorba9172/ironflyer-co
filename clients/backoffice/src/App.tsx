import { Routes, Route } from 'react-router-dom';
import { AppShell } from './AppShell';
import { Overview } from './pages/Overview';
import { Projects } from './pages/Projects';
import { Wallet } from './pages/Wallet';
import { Audit } from './pages/Audit';

// Internal operator admin: revenue, projects, wallet/spend, and audit —
// all rendered inside the persistent sidebar shell.
export function App() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route path="/" element={<Overview />} />
        <Route path="/projects" element={<Projects />} />
        <Route path="/wallet" element={<Wallet />} />
        <Route path="/audit" element={<Audit />} />
      </Route>
    </Routes>
  );
}
