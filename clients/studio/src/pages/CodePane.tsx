import { useMemo, useState } from 'react';
import { Box, Stack, Typography } from '@mui/material';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { CodeEditor } from '@ironflyer/ui-web/fx';
import { useThemeMode } from '@ironflyer/ui-web';
import { useLiveProjectId } from '../hooks/useLiveProjectId';

interface CodeFile { path: string; size: number; language: string; content?: string | null }

const SAMPLE: CodeFile[] = [
  { path: 'src/App.tsx', size: 0, language: 'tsx', content: "import { useState } from 'react';\n\nexport default function App() {\n  const [count, setCount] = useState(0);\n  return <button onClick={() => setCount((c) => c + 1)}>count is {count}</button>;\n}\n" },
  { path: 'README.md', size: 0, language: 'md', content: '# Project\n\nConnect the orchestrator to load your real files here.\n' },
];

function langOf(path: string): string {
  if (/\.json$/i.test(path)) return 'json';
  return 'javascript';
}

// Code surface — CodeMirror 6 (not VS Code). Lists the project's files and
// opens them in a light, fast editor. Save lands once the runtime File API is wired.
export function CodePane() {
  const liveProjectId = useLiveProjectId();
  const { mode } = useThemeMode();
  const [selected, setSelected] = useState(0);

  const { data: files } = useGraphQLQuery<CodeFile[], { projectFiles: CodeFile[] }>({
    key: ['code-files', liveProjectId ?? 'none'],
    operationName: 'ProjectFiles', query: operations.PROJECT_FILES,
    variables: { id: liveProjectId }, fallbackData: SAMPLE, enabled: !!liveProjectId,
    map: (r) => (r.projectFiles?.length ? r.projectFiles : SAMPLE),
  });

  const file = files[Math.min(selected, files.length - 1)] ?? files[0];
  const value = useMemo(() => file?.content ?? '// (empty file)\n', [file]);

  return (
    <Box sx={{ flex: 1, height: '100%', display: 'flex', minWidth: 0, bgcolor: 'background.default' }}>
      {/* file list */}
      <Box sx={{ width: 240, flexShrink: 0, borderRight: 1, borderColor: 'divider', overflowY: 'auto', py: 1 }}>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.66rem', letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled', px: 2, py: 1 })}>Files</Typography>
        {files.map((f, i) => (
          <Box
            key={f.path}
            onClick={() => setSelected(i)}
            sx={{ px: 2, py: 0.75, cursor: 'pointer', bgcolor: i === selected ? 'action.selected' : 'transparent', '&:hover': { bgcolor: 'action.hover' } }}
          >
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.8rem', color: i === selected ? 'text.primary' : 'text.secondary' })} noWrap>{f.path}</Typography>
          </Box>
        ))}
      </Box>

      {/* editor */}
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 2, py: 1, borderBottom: 1, borderColor: 'divider' }}>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.8rem' })}>{file?.path}</Typography>
          <Typography sx={{ fontSize: '0.72rem', color: 'text.disabled' }}>read-only · save lands with the runtime File API</Typography>
        </Stack>
        <Box sx={{ flex: 1, minHeight: 0, overflow: 'auto', '& .cm-editor': { height: '100%' } }}>
          {file && <CodeEditor value={value} language={langOf(file.path)} dark={mode === 'dark'} readOnly height="100%" />}
        </Box>
      </Box>
    </Box>
  );
}
