import { useState } from 'react';
import { Box } from '@mui/material';
import { AnimatePresence, motion, confirmAction, toast } from '@ironflyer/ui-web/fx';
import { EditorTopBar, type EditorTab } from '../components/EditorTopBar';
import { ChatPanel } from '../components/ChatPanel';
import { PreviewPane } from '../components/PreviewPane';
import { DashboardPane } from '../components/DashboardPane';
import { GateMap } from '../components/GateMap';
import { SecurityPane } from '../components/SecurityPane';
import { GateInspector } from '../components/GateInspector';
import { useStudio } from '../store';

export function Editor() {
  const project = useStudio((s) => s.current);
  const initialPrompt = useStudio((s) => s.initialPrompt);
  const [tab, setTab] = useState<EditorTab>('dashboard');

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
            {tab === 'preview' && <PreviewPane />}
            {tab === 'map' && <GateMap project={project} />}
            {tab === 'security' && <SecurityPane security={project.security} />}
            {tab === 'dashboard' && <DashboardPane projectId={project.id} fallback={project} />}
          </motion.div>
        </AnimatePresence>
      </Box>
      <GateInspector />
    </Box>
  );
}
