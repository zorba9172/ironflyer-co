import { useState } from 'react';
import { Box } from '@mui/material';
import { EditorTopBar, type EditorTab } from '../components/EditorTopBar';
import { ChatPanel } from '../components/ChatPanel';
import { PreviewPane } from '../components/PreviewPane';
import { DashboardPane } from '../components/DashboardPane';
import { useStudio } from '../store';

export function Editor() {
  const project = useStudio((s) => s.current);
  const initialPrompt = useStudio((s) => s.initialPrompt);
  const [tab, setTab] = useState<EditorTab>('dashboard');
  return (
    <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
      <EditorTopBar projectName={project.name} tab={tab} onTab={setTab} />
      <Box sx={{ flex: 1, display: 'flex', minHeight: 0 }}>
        <ChatPanel initialPrompt={initialPrompt} />
        {tab === 'preview' ? <PreviewPane /> : <DashboardPane projectId={project.id} fallback={project} />}
      </Box>
    </Box>
  );
}
