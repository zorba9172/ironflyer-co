import { lazy, Suspense, useState } from 'react';
import { Box, CircularProgress } from '@mui/material';
import { AnimatePresence, motion, confirmAction, toast } from '@ironflyer/ui-web/fx';
import { EditorTopBar, type EditorTab } from '../components/EditorTopBar';
import { ChatPanel } from '../components/ChatPanel';
import { GateInspector } from '../components/GateInspector';
import { useStudio } from '../store';

// Each pane is code-split: opening a tab is the only thing that pulls its heavy
// deps (echarts, ag-grid, monaco, react-flow) — the studio boots on the chat +
// shell alone.
const PreviewPane = lazy(() => import('../components/PreviewPane').then((m) => ({ default: m.PreviewPane })));
const DashboardPane = lazy(() => import('../components/DashboardPane').then((m) => ({ default: m.DashboardPane })));
const GateMap = lazy(() => import('../components/GateMap').then((m) => ({ default: m.GateMap })));
const SecurityPane = lazy(() => import('../components/SecurityPane').then((m) => ({ default: m.SecurityPane })));
const DocumentsPane = lazy(() => import('./DocumentsPane').then((m) => ({ default: m.DocumentsPane })));
const LogsPane = lazy(() => import('./LogsPane').then((m) => ({ default: m.LogsPane })));
const CodePane = lazy(() => import('./CodePane').then((m) => ({ default: m.CodePane })));
const PerformancePane = lazy(() => import('./PerformancePane').then((m) => ({ default: m.PerformancePane })));
const QualityPane = lazy(() => import('./QualityPane').then((m) => ({ default: m.QualityPane })));
const AgentsManagerPane = lazy(() => import('./AgentsManagerPane').then((m) => ({ default: m.AgentsManagerPane })));
const ExecutionTeamGraph = lazy(() => import('./ExecutionTeamGraph').then((m) => ({ default: m.ExecutionTeamGraph })));
const TheaterPane = lazy(() => import('./TheaterPane').then((m) => ({ default: m.TheaterPane })));
// Operate group — post-deploy surfaces, each code-split.
const DataPane = lazy(() => import('./DataPane').then((m) => ({ default: m.DataPane })));
const UsersPane = lazy(() => import('./UsersPane').then((m) => ({ default: m.UsersPane })));
const AnalyticsPane = lazy(() => import('./AnalyticsPane').then((m) => ({ default: m.AnalyticsPane })));
const DomainsPane = lazy(() => import('./DomainsPane').then((m) => ({ default: m.DomainsPane })));
const AutomationsPane = lazy(() => import('./AutomationsPane').then((m) => ({ default: m.AutomationsPane })));
const ApiPane = lazy(() => import('./ApiPane').then((m) => ({ default: m.ApiPane })));
const MarketingPane = lazy(() => import('./MarketingPane').then((m) => ({ default: m.MarketingPane })));
const SettingsPane = lazy(() => import('./SettingsPane').then((m) => ({ default: m.SettingsPane })));

function PaneFallback() {
  return (
    <Box sx={{ flex: 1, display: 'grid', placeItems: 'center', bgcolor: 'background.default' }}>
      <CircularProgress size={26} thickness={5} />
    </Box>
  );
}

export function Editor() {
  const project = useStudio((s) => s.current);
  const initialPrompt = useStudio((s) => s.initialPrompt);
  // Land on the live preview when a scaffold is already seeded (instant-start
  // from a template) so the running app is the first thing the operator sees.
  const seededFiles = useStudio((s) => s.generatedFiles.length > 0);
  const [tab, setTab] = useState<EditorTab>(seededFiles ? 'theater' : 'dashboard');

  const open = project.gates.filter((g) => g.blocking).length;
  const remaining = project.meters.walletBudget - project.meters.walletUsed;
  const onDeploy = async () => {
    // Hard economic law 1: no execution starts without budget → 402.
    if (remaining <= 0 || project.profitGuard.verdict === 'block') {
      await confirmAction({
        title: 'Top up required (402)',
        text: `Deploy reserves funds before it runs. Wallet has $${remaining.toFixed(2)} of $${project.meters.walletBudget} left.`,
        confirmText: 'Top up wallet',
        danger: true,
      });
      return;
    }
    if (open > 0) {
      const go = await confirmAction({
        title: `${open} gate${open > 1 ? 's' : ''} still open`,
        text: 'Deploying now ships with unclosed finisher gates. Continue anyway?',
        confirmText: 'Deploy anyway',
        danger: true,
      });
      if (!go) return;
    }
    toast('Deploy queued — watch the board for status.', 'success');
  };

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <EditorTopBar projectName={project.name} tab={tab} onTab={setTab} onDeploy={onDeploy} />
      <Box sx={{ flex: 1, display: 'flex', minHeight: 0 }}>
        <ChatPanel initialPrompt={initialPrompt} />
        <AnimatePresence mode="wait">
          <motion.div
            key={tab}
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
            style={{ flex: 1, display: 'flex', minWidth: 0 }}
          >
            <Suspense fallback={<PaneFallback />}>
              {tab === 'preview' && <PreviewPane />}
              {tab === 'theater' && <TheaterPane project={project} />}
              {tab === 'map' && <GateMap project={project} onOpenTab={setTab} />}
              {tab === 'security' && <SecurityPane fallback={project.security} />}
              {tab === 'code' && <CodePane />}
              {tab === 'dashboard' && <DashboardPane projectId={project.id} fallback={project} />}
              {tab === 'agents' && <AgentsManagerPane project={project} />}
              {tab === 'team' && <ExecutionTeamGraph project={project} />}
              {tab === 'performance' && <PerformancePane />}
              {tab === 'quality' && <QualityPane />}
              {tab === 'logs' && <LogsPane fallback={project} />}
              {tab === 'documents' && <DocumentsPane />}
              {tab === 'data' && <DataPane />}
              {tab === 'users' && <UsersPane />}
              {tab === 'analytics' && <AnalyticsPane />}
              {tab === 'domains' && <DomainsPane />}
              {tab === 'automations' && <AutomationsPane />}
              {tab === 'api' && <ApiPane />}
              {tab === 'marketing' && <MarketingPane />}
              {tab === 'settings' && <SettingsPane />}
            </Suspense>
          </motion.div>
        </AnimatePresence>
      </Box>
      <GateInspector />
    </Box>
  );
}
