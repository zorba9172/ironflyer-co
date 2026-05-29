import { lazy, Suspense, useState } from 'react';
import { Box, CircularProgress } from '@mui/material';
import { AnimatePresence, motion, confirmAction } from '@ironflyer/ui-web/fx';
import { formatUSD } from '@ironflyer/core';
import { EditorTopBar, type EditorTab, type EditorDeployReadiness } from '../components/EditorTopBar';
import { ChatPanel } from '../components/ChatPanel';
import { GateInspector } from '../components/GateInspector';
import { useStudio } from '../store';
import { useWallet } from '../hooks/useEconomics';
import { useDispatchAgent } from '../hooks/useDispatchAgent';
import { useLiveGates } from '../hooks/useLiveGates';

// Each pane is code-split: opening a tab is the only thing that pulls its heavy
// deps (echarts, ag-grid, monaco, react-flow) — the studio boots on the chat +
// shell alone.
const DashboardPane = lazy(() => import('../components/DashboardPane').then((m) => ({ default: m.DashboardPane })));
const GateMap = lazy(() => import('../components/GateMap').then((m) => ({ default: m.GateMap })));
const SecurityPane = lazy(() => import('../components/SecurityPane').then((m) => ({ default: m.SecurityPane })));
const DocumentsPane = lazy(() => import('./DocumentsPane').then((m) => ({ default: m.DocumentsPane })));
const LogsPane = lazy(() => import('./LogsPane').then((m) => ({ default: m.LogsPane })));
const CodePane = lazy(() => import('./CodePane').then((m) => ({ default: m.CodePane })));
// Consolidated workspaces — each fronts a cluster of naturally-related views as
// inner tabs (Quality → Review | Code health | Coverage | Performance;
// Execution team → Roster | Graph), keeping the top menu lean.
const QualityWorkspace = lazy(() => import('./QualityWorkspace').then((m) => ({ default: m.QualityWorkspace })));
const ExecutionTeamWorkspace = lazy(() => import('./ExecutionTeamWorkspace').then((m) => ({ default: m.ExecutionTeamWorkspace })));
const PreviewWorkspace = lazy(() => import('./PreviewWorkspace').then((m) => ({ default: m.PreviewWorkspace })));
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
  const [tab, setTab] = useState<EditorTab>(seededFiles ? 'preview' : 'dashboard');

  const { wallet, isLive: walletLive } = useWallet();
  const { dispatch } = useDispatchAgent();
  const { gates: liveGates } = useLiveGates();

  // Gate the deploy on the LIVE wallet + gates when connected; fall back to the
  // session's sample meters offline so the 402 demo still reads correctly.
  const openGates = liveGates.filter((g) => g.blocking);
  const open = openGates.length;
  const remaining = walletLive ? wallet.availableUSD : project.meters.walletBudget - project.meters.walletUsed;
  const blocked = walletLive ? wallet.availableUSD <= 0 : project.profitGuard.verdict === 'block';
  const walletReserve = project.profitGuard.reservedUSD;
  const deployReadiness: EditorDeployReadiness = blocked
    ? { tone: 'error', label: 'Wallet blocked', detail: `${formatUSD(remaining)} available for the next deploy reservation.` }
    : open > 0
      ? { tone: 'warning', label: `${open} gate${open > 1 ? 's' : ''} open`, detail: openGates.slice(0, 3).map((g) => `${g.name}: ${g.blocking}`).join(' | ') }
      : { tone: 'success', label: 'Deploy ready', detail: `Wallet has ${formatUSD(remaining)} available; rollback review runs through the deploy gate.` };

  const deployPreflightText = () => {
    const gateLine = open === 0
      ? 'Open gates: none blocking deploy.'
      : `Open gates: ${open} blocking (${openGates.slice(0, 3).map((g) => `${g.name}: ${g.blocking}`).join('; ')}${open > 3 ? `; +${open - 3} more` : ''}).`;
    const reserveLine = walletReserve > 0
      ? `Wallet reserve: ${formatUSD(walletReserve)} estimated hold, ${formatUSD(remaining)} available.`
      : `Wallet reserve: orchestrator-held on dispatch, ${formatUSD(remaining)} available.`;
    return [
      reserveLine,
      gateLine,
      'Review intent: keep generated patches behind the finisher review gates before promotion.',
      'Rollback intent: deploy with health checks and keep the previous production version ready.',
    ].join('\n');
  };

  const onDeploy = async () => {
    // Hard economic law 1: no execution starts without budget → 402.
    if (remaining <= 0 || blocked) {
      await confirmAction({
        title: 'Top up required (402)',
        text: `Deploy reserves funds before it runs. Wallet has ${formatUSD(remaining)} available. Review the open gates, then top up before dispatching.`,
        confirmText: 'Top up wallet',
        danger: true,
      });
      return;
    }
    const go = await confirmAction({
      title: open > 0 ? `Review deploy: ${open} gate${open > 1 ? 's' : ''} open` : 'Review deploy preflight',
      text: deployPreflightText(),
      confirmText: open > 0 ? 'Reserve & deploy anyway' : 'Reserve & deploy',
      danger: open > 0,
    });
    if (!go) return;
    // Deploy runs through the finisher's Deploy gate — dispatch the real loop
    // (offline this surfaces an honest "connect the orchestrator" note).
    await dispatch('deploy');
  };

  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <EditorTopBar projectName={project.name} tab={tab} onTab={setTab} onDeploy={onDeploy} deployReadiness={deployReadiness} />
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
              {tab === 'preview' && <PreviewWorkspace project={project} />}
              {tab === 'map' && <GateMap project={project} onOpenTab={setTab} />}
              {tab === 'security' && <SecurityPane fallback={project.security} />}
              {tab === 'code' && <CodePane />}
              {tab === 'dashboard' && <DashboardPane projectId={project.id} fallback={project} />}
              {tab === 'team' && <ExecutionTeamWorkspace project={project} />}
              {tab === 'quality' && <QualityWorkspace />}
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
