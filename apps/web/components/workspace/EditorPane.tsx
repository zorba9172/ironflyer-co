'use client';

// EditorPane — Monaco editor wired to the runtime File API. Loads files
// lazily so the heavy editor only mounts when the tab is opened.

import dynamic from 'next/dynamic';
import { useEffect, useState, ComponentType } from 'react';
import { Box, Button, Skeleton, Stack, Tooltip, Typography } from '@mui/material';
import { Save } from '@mui/icons-material';
import { runtime, Workspace } from '../../lib/runtime';
import { tokens } from '../../lib/theme';

// Monaco is heavy — load only on the client when this pane is mounted.
// The package may not yet be installed locally (it's listed in package.json
// but the actual install happens at deploy time), so we declare the prop
// shape and cast through `any` to keep the typecheck green.
interface MonacoProps {
  height?: string | number;
  value?: string;
  language?: string;
  theme?: string;
  onChange?: (value: string | undefined) => void;
  options?: Record<string, unknown>;
}
const MonacoEditor = dynamic<MonacoProps>(
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore — module is added to package.json; resolves at install time.
  () => import('@monaco-editor/react').then((m: { default: ComponentType<MonacoProps> }) => m.default),
  {
    ssr: false,
    loading: () => <Skeleton variant="rounded" sx={{ width: '100%', height: '100%', minHeight: 320 }} />,
  },
) as ComponentType<MonacoProps>;

interface Props {
  workspace: Workspace | null;
  selectedFile: string | null;
}

export function EditorPane({ workspace, selectedFile }: Props) {
  const [content, setContent] = useState<string>('');
  const [original, setOriginal] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!workspace || !selectedFile) {
      setContent(''); setOriginal(''); setError(null);
      return;
    }
    let alive = true;
    setLoading(true); setError(null);
    runtime.readFile(workspace.id, selectedFile)
      .then((t) => {
        if (!alive) return;
        const text = t.length > 500_000 ? t.slice(0, 500_000) + '\n…truncated' : t;
        setContent(text); setOriginal(text);
      })
      .catch((e) => alive && setError(String(e?.message ?? e)))
      .finally(() => alive && setLoading(false));
    return () => { alive = false; };
  }, [workspace?.id, selectedFile]);

  const dirty = content !== original;

  async function save() {
    if (!workspace || !selectedFile || !dirty) return;
    setSaving(true); setError(null);
    try {
      await runtime.writeFile(workspace.id, selectedFile, content);
      setOriginal(content);
    } catch (e) {
      setError(String((e as Error)?.message ?? e));
    } finally {
      setSaving(false);
    }
  }

  if (!workspace) {
    return (
      <EmptyEditor
        title="אין סביבת ריצה"
        body="הריצו את ה־Finisher כדי להפעיל סביבה ולערוך קבצים בלייב."
      />
    );
  }
  if (!selectedFile) {
    return (
      <EmptyEditor
        title="בחרו קובץ מהעץ"
        body="לחיצה על קובץ ברשימה משמאל תפתח אותו כאן בעורך מלא."
      />
    );
  }

  return (
    <Stack spacing={1} sx={{ height: '100%', minHeight: 0 }}>
      <Stack direction="row" alignItems="center" spacing={1}>
        <Typography
          variant="caption"
          sx={{
            flex: 1, minWidth: 0, fontFamily: tokens.font.mono,
            color: tokens.color.text.secondary,
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          }}
          title={selectedFile}
        >
          {selectedFile}
          {dirty && (
            <Box component="span" sx={{ ml: 0.8, color: tokens.color.accent.warning, fontWeight: 900 }}>
              ●
            </Box>
          )}
        </Typography>
        <Tooltip title={dirty ? 'שמירה לסביבת הריצה' : 'אין שינויים'}>
          <span>
            <Button
              startIcon={<Save fontSize="small" />}
              size="small" variant="contained"
              disabled={!dirty || saving}
              onClick={save}
              sx={{ borderRadius: '10px' }}
            >
              {saving ? 'שומר…' : 'שמירה'}
            </Button>
          </span>
        </Tooltip>
      </Stack>

      <Box sx={{
        flex: 1, minHeight: 0,
        borderRadius: '10px', overflow: 'hidden',
        border: '1px solid rgba(17,17,17,0.12)',
        bgcolor: '#0d0e0f',
      }}>
        {loading ? (
          <Skeleton variant="rectangular" sx={{ width: '100%', height: '100%' }} />
        ) : error ? (
          <Box sx={{ p: 2 }}>
            <Typography variant="body2" sx={{ color: tokens.color.accent.danger, fontWeight: 700 }}>
              לא הצלחנו לטעון את הקובץ
            </Typography>
            <Typography variant="caption" color="text.secondary">{error}</Typography>
          </Box>
        ) : (
          <MonacoEditor
            height="100%"
            value={content}
            onChange={(v) => setContent(v ?? '')}
            language={languageForPath(selectedFile)}
            theme="vs-dark"
            options={{
              minimap: { enabled: false },
              fontSize: 13,
              wordWrap: 'on',
              automaticLayout: true,
              scrollBeyondLastLine: false,
              padding: { top: 12, bottom: 12 },
            }}
          />
        )}
      </Box>
    </Stack>
  );
}

function EmptyEditor({ title, body }: { title: string; body: string }) {
  return (
    <Stack spacing={1.1} alignItems="center" sx={{
      height: '100%', textAlign: 'center', justifyContent: 'center', py: 6,
    }}>
      <Typography variant="subtitle1" sx={{ fontWeight: 800 }}>{title}</Typography>
      <Typography variant="body2" sx={{ color: tokens.color.text.muted, maxWidth: 320 }}>{body}</Typography>
    </Stack>
  );
}

function languageForPath(path: string): string {
  const lower = path.toLowerCase();
  const dot = lower.lastIndexOf('.');
  if (dot < 0) {
    if (lower.endsWith('dockerfile')) return 'dockerfile';
    if (lower.endsWith('makefile')) return 'makefile';
    return 'plaintext';
  }
  const ext = lower.slice(dot + 1);
  switch (ext) {
    case 'ts': case 'tsx': return 'typescript';
    case 'js': case 'jsx': return 'javascript';
    case 'json':           return 'json';
    case 'go':             return 'go';
    case 'py':             return 'python';
    case 'rb':             return 'ruby';
    case 'rs':             return 'rust';
    case 'java':           return 'java';
    case 'kt':             return 'kotlin';
    case 'swift':          return 'swift';
    case 'c': case 'h':    return 'c';
    case 'cpp': case 'hpp': case 'cc': return 'cpp';
    case 'cs':             return 'csharp';
    case 'css': case 'scss': case 'sass': return 'css';
    case 'html': case 'htm': return 'html';
    case 'md':             return 'markdown';
    case 'yaml': case 'yml': return 'yaml';
    case 'toml':           return 'toml';
    case 'xml':            return 'xml';
    case 'sh': case 'bash': case 'zsh': return 'shell';
    case 'sql':            return 'sql';
    case 'graphql': case 'gql': return 'graphql';
    case 'svg':            return 'xml';
    default:               return 'plaintext';
  }
}
