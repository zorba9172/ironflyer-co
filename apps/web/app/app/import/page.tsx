'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
  CloudDownload, GitHub, OpenInNew, PlayArrow, Tune,
} from '@mui/icons-material';
import {
  Box, Button, Chip, CircularProgress, FormControlLabel, LinearProgress, Stack,
  Switch, TextField, Typography,
} from '@mui/material';
import { tokens } from '../../../lib/theme';
import { RequireAuth, useAuth } from '../../auth-context';
import { AppShell, PageTitle, Surface } from '../workspace-shell';
import { githubApi, GitHubStatus } from '../../../lib/github';
import {
  ImportEvent, ImportResult, startImport,
} from '../../../lib/api/import';

interface LogLine {
  id: number;
  level: 'info' | 'warn' | 'error' | 'success';
  text: string;
  ts: string;
}

export default function ImportPage() {
  return (
    <RequireAuth>
      <ImportInner />
    </RequireAuth>
  );
}

// Validates a GitHub repo URL or `owner/repo` shorthand. Returns a
// human-readable Hebrew error when invalid.
function validateRepoURL(input: string): string | null {
  const s = input.trim();
  if (!s) return 'הכנס כתובת GitHub חוקית';
  // owner/repo shorthand.
  if (/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(s)) return null;
  try {
    const u = new URL(s);
    if (u.protocol !== 'https:') return 'הכתובת חייבת להיות מסוג https://';
    if (u.host.toLowerCase() !== 'github.com') return 'נתמכות רק כתובות מ-github.com';
    const path = u.pathname.replace(/^\/+|\/+$/g, '').replace(/\.git$/, '');
    const parts = path.split('/');
    if (parts.length < 2 || !parts[0] || !parts[1]) return 'הכתובת חייבת לכלול owner/repo';
    return null;
  } catch {
    return 'הכנס כתובת GitHub חוקית';
  }
}

function ImportInner() {
  const { user, logout } = useAuth();
  const router = useRouter();

  // Form state.
  const [repoUrl, setRepoUrl] = useState('');
  const [repoError, setRepoError] = useState<string | null>(null);
  const [branch, setBranch] = useState('');
  const [subdir, setSubdir] = useState('');
  const [makePublic, setMakePublic] = useState(false);
  const [advanced, setAdvanced] = useState(false);

  // GitHub connection status — used to hint about private repos.
  const [github, setGithub] = useState<GitHubStatus | null>(null);
  const [githubLoaded, setGithubLoaded] = useState(false);

  // Pipeline state.
  const [running, setRunning] = useState(false);
  const [logs, setLogs] = useState<LogLine[]>([]);
  const [currentStage, setCurrentStage] = useState<string>('');
  const [result, setResult] = useState<ImportResult | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const ctrlRef = useRef<AbortController | null>(null);
  const logIdRef = useRef(0);
  const logScrollRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    void githubApi.me().then((s) => setGithub(s)).catch(() => setGithub({ connected: false })).finally(() => setGithubLoaded(true));
  }, []);

  useEffect(() => {
    if (logScrollRef.current) {
      logScrollRef.current.scrollTop = logScrollRef.current.scrollHeight;
    }
  }, [logs]);

  const pushLog = useCallback((level: LogLine['level'], text: string) => {
    logIdRef.current += 1;
    setLogs((prev) => [
      ...prev,
      {
        id: logIdRef.current,
        level,
        text,
        ts: new Date().toLocaleTimeString('he-IL'),
      },
    ]);
  }, []);

  const stageLabel = useMemo<Record<string, string>>(() => ({
    import_started: 'מתחיל יבוא...',
    project_created: 'נוצרה רשומת פרויקט',
    cloning: 'משכפל את הריפו לסביבת הריצה...',
    cloned: 'הריפו שוכפל בהצלחה',
    detecting_stack: 'מזהה את ה-stack...',
    stack_detected: 'ה-stack זוהה',
    warning: 'אזהרה',
    ready: 'הייבוא הסתיים',
    failed: 'הייבוא נכשל',
  }), []);

  function startPipeline() {
    const err = validateRepoURL(repoUrl);
    setRepoError(err);
    if (err) return;
    setLogs([]);
    setResult(null);
    setFailure(null);
    setCurrentStage('import_started');
    setRunning(true);
    pushLog('info', `שולח בקשת יבוא עבור ${repoUrl.trim()}`);

    const ctrl = startImport(
      {
        repoUrl: repoUrl.trim(),
        branch: branch.trim() || undefined,
        subdir: subdir.trim() || undefined,
        makePublic,
      },
      {
        onEvent: (evt: ImportEvent) => {
          setCurrentStage(evt.type);
          const label = stageLabel[evt.type] ?? evt.type;
          if (evt.type === 'warning' && evt.warning) {
            pushLog('warn', `${label}: ${evt.warning}`);
          } else if (evt.type === 'stack_detected' && evt.stack) {
            pushLog(
              'success',
              `${label}: ${evt.stack.frontend} · ${evt.stack.backend} · ${evt.stack.storage} · ${evt.stack.auth}`,
            );
          } else if (evt.message) {
            pushLog('info', `${label}: ${evt.message}`);
          } else {
            pushLog('info', label);
          }
        },
        onResult: (res: ImportResult) => {
          setResult(res);
          pushLog('success', `פרויקט מוכן: ${res.projectId}`);
        },
        onError: (msg: string) => {
          setFailure(msg);
          pushLog('error', msg);
        },
        onClose: () => {
          setRunning(false);
          ctrlRef.current = null;
        },
      },
    );
    ctrlRef.current = ctrl;
  }

  function cancelPipeline() {
    ctrlRef.current?.abort();
    ctrlRef.current = null;
    setRunning(false);
    pushLog('warn', 'המשתמש ביטל את הייבוא');
  }

  function openProject() {
    if (!result) return;
    const initialPrompt = encodeURIComponent(
      `יובא הריפו ${repoUrl.trim()}. בוא נמשיך לעבוד עליו — הצע את הצעדים הבאים.`,
    );
    router.push(`/app/projects/${result.projectId}?initialPrompt=${initialPrompt}`);
  }

  const progressPct = useMemo(() => {
    const order = [
      '', 'import_started', 'project_created', 'cloning', 'cloned',
      'detecting_stack', 'stack_detected', 'ready',
    ];
    const idx = order.indexOf(currentStage);
    if (idx <= 0) return running ? 6 : 0;
    return Math.min(100, Math.round((idx / (order.length - 1)) * 100));
  }, [currentStage, running]);

  return (
    <AppShell userEmail={user?.email ?? 'workspace'} onLogout={logout}>
      <PageTitle
        eyebrow="ייבוא"
        title="ייבא ריפוזיטורי מ-GitHub"
        subtitle="הבא את הקוד שלך כפי שהוא, ו-Ironflyer ימשיך לסיים אותו דרך השערים: עיצוב, ארכיטקטורה, איכות, פריסה."
      />

      {githubLoaded && !github?.connected && (
        <Surface sx={{ p: 1.6, mb: 1.6, borderRight: `3px solid ${tokens.color.accent.lime}` }}>
          <Stack direction="row" spacing={1.2} alignItems="center">
            <GitHub />
            <Box sx={{ flex: 1 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 800 }}>
                התחבר ל-GitHub כדי לייבא ריפוזיטוריות פרטיות
              </Typography>
              <Typography variant="body2" color="text.secondary">
                ריפוזיטוריות ציבוריות עובדות גם בלי חיבור — אבל לפרטיות נחוצה הסכמת OAuth.
              </Typography>
            </Box>
            <Button
              variant="outlined"
              size="small"
              component={Link}
              href="/app/connectors"
              endIcon={<OpenInNew fontSize="small" />}
            >
              עבור לחיבורים
            </Button>
          </Stack>
        </Surface>
      )}

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.05fr 0.95fr' }, gap: 1.6 }}>
        <Surface sx={{ p: 2.2 }}>
          <Stack direction="row" spacing={1.2} alignItems="center" sx={{ mb: 1.4 }}>
            <Box sx={{
              width: 42, height: 42, borderRadius: '8px', display: 'grid', placeItems: 'center',
              bgcolor: tokens.color.accent.lime, color: tokens.color.text.inverse,
            }}>
              <CloudDownload />
            </Box>
            <Box>
              <Typography variant="h6" sx={{ fontWeight: 900 }}>פרטי הריפו</Typography>
              <Typography variant="body2" color="text.secondary">
                אפשר להדביק כתובת מלאה או קיצור בסגנון <code>owner/repo</code>.
              </Typography>
            </Box>
          </Stack>

          <TextField
            label="כתובת GitHub"
            placeholder="https://github.com/owner/repo או owner/repo"
            fullWidth
            value={repoUrl}
            onChange={(e) => {
              setRepoUrl(e.target.value);
              if (repoError) setRepoError(null);
            }}
            onBlur={() => setRepoError(validateRepoURL(repoUrl))}
            error={!!repoError}
            helperText={repoError ?? ' '}
            disabled={running}
            sx={{ mb: 1.2 }}
          />

          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: advanced ? 1.2 : 0.4 }}>
            <Button
              size="small"
              variant="text"
              startIcon={<Tune fontSize="small" />}
              onClick={() => setAdvanced((v) => !v)}
            >
              {advanced ? 'הסתר אפשרויות מתקדמות' : 'אפשרויות מתקדמות'}
            </Button>
          </Stack>

          {advanced && (
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.2} sx={{ mb: 1.2 }}>
              <TextField
                label="ענף (ברירת מחדל: main)"
                placeholder="main"
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                disabled={running}
                fullWidth
              />
              <TextField
                label="תת-תיקייה (אופציונלי)"
                placeholder="apps/web"
                value={subdir}
                onChange={(e) => setSubdir(e.target.value)}
                disabled={running}
                fullWidth
              />
            </Stack>
          )}

          <FormControlLabel
            sx={{ mb: 1.6 }}
            control={
              <Switch
                checked={makePublic}
                onChange={(_, v) => setMakePublic(v)}
                disabled={running}
              />
            }
            label="הפוך לציבורי"
          />

          <Stack direction="row" spacing={1}>
            <Button
              variant="contained"
              startIcon={running ? <CircularProgress size={16} color="inherit" /> : <PlayArrow />}
              onClick={startPipeline}
              disabled={running || !repoUrl.trim()}
            >
              {running ? 'מייבא...' : 'התחל ייבוא'}
            </Button>
            {running && (
              <Button variant="outlined" onClick={cancelPipeline}>
                בטל
              </Button>
            )}
          </Stack>
        </Surface>

        <Surface sx={{ p: 2.2, display: 'flex', flexDirection: 'column', minHeight: 320 }}>
          <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 1 }}>
            <Typography variant="h6" sx={{ fontWeight: 900 }}>לוג ייבוא</Typography>
            <Chip
              size="small"
              label={running ? 'בהרצה' : result ? 'מוכן' : failure ? 'נכשל' : 'מוכן להתחלה'}
              sx={{
                bgcolor: running
                  ? tokens.color.accent.lime
                  : failure
                  ? '#ffd5d5'
                  : result
                  ? tokens.color.accent.lime
                  : '#fffaf1',
                fontWeight: 800,
              }}
            />
          </Stack>
          <LinearProgress
            variant="determinate"
            value={progressPct}
            sx={{
              height: 6, borderRadius: '999px', mb: 1.4,
              bgcolor: 'rgba(17,17,17,0.08)',
              '& .MuiLinearProgress-bar': { bgcolor: tokens.color.accent.lime },
            }}
          />
          <Box
            ref={logScrollRef}
            sx={{
              flex: 1, minHeight: 200, maxHeight: 360, overflowY: 'auto',
              bgcolor: '#0d0d0d', color: '#e7e2d4', borderRadius: '8px',
              fontFamily: tokens.font.mono, fontSize: 12.5, lineHeight: 1.6,
              p: 1.4,
            }}
          >
            {logs.length === 0 ? (
              <Typography variant="body2" sx={{ color: '#86807a', fontFamily: tokens.font.mono }}>
                הלוג יופיע כאן כשהייבוא יתחיל.
              </Typography>
            ) : (
              logs.map((line) => (
                <Box key={line.id} sx={{
                  color: line.level === 'error' ? '#ff8a8a'
                    : line.level === 'warn' ? '#ffd166'
                    : line.level === 'success' ? tokens.color.accent.lime
                    : '#e7e2d4',
                }}>
                  <span style={{ opacity: 0.55 }}>[{line.ts}]</span> {line.text}
                </Box>
              ))
            )}
          </Box>

          {result && (
            <Stack direction="row" spacing={1} sx={{ mt: 1.6 }}>
              <Button variant="contained" onClick={openProject} endIcon={<OpenInNew fontSize="small" />}>
                פתח את הפרויקט
              </Button>
              <Chip
                label={`stack: ${result.stack.frontend} · ${result.stack.backend}`}
                sx={{ bgcolor: '#fffaf1', fontWeight: 700 }}
              />
            </Stack>
          )}
          {failure && !result && (
            <Typography variant="body2" sx={{ color: '#9b1010', mt: 1.4 }}>
              {failure}
            </Typography>
          )}
        </Surface>
      </Box>
    </AppShell>
  );
}
