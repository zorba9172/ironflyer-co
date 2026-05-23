'use client';

// FigmaImportPanel — pulls a Figma file into the project workspace. Given a
// pasted Figma URL or a raw 20-30 char file key, calls the orchestrator's
// figma-import flow which extracts design tokens (colors / typography /
// spacing / radii), an inventory of top-level frames/components, and
// optional PNG thumbnails. The result is rendered as a two-tab summary
// (Tokens / Frames) right in the workspace so the user can confirm the
// extraction before kicking off a run.

import { useCallback, useMemo, useState } from 'react';
import {
  Box, Button, CircularProgress, Stack, Tab, Tabs, TextField, Typography,
} from '@mui/material';
import { CloudDownload, Palette } from '@mui/icons-material';

import { api, extractFigmaFileKey, FigmaImportResult, FigmaTypographyToken } from '../../lib/api';
import { tokens } from '../../lib/theme';

interface Props {
  projectId: string;
  workspaceId?: string | null;
  onImported?: (summary: { tokenCount: number; componentCount: number }) => void;
}

// Accept either a Figma URL or a raw key — keys are 20-30 chars of
// [A-Za-z0-9]. The orchestrator validates server-side too, but rejecting
// obvious junk client-side keeps the network round-trip honest.
const RAW_KEY_RE = /^[A-Za-z0-9]{20,30}$/;

function resolveFileKey(input: string): string | null {
  const trimmed = input.trim();
  if (!trimmed) return null;
  const fromUrl = extractFigmaFileKey(trimmed);
  if (fromUrl) return fromUrl;
  if (RAW_KEY_RE.test(trimmed)) return trimmed;
  return null;
}

export function FigmaImportPanel({ projectId, workspaceId, onImported }: Props) {
  const [input, setInput] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<FigmaImportResult | null>(null);
  const [tab, setTab] = useState<'tokens' | 'frames'>('tokens');

  const resolvedKey = useMemo(() => resolveFileKey(input), [input]);
  const canSubmit = Boolean(resolvedKey && workspaceId) && !busy;

  const tokenCount = useMemo(() => {
    if (!result) return 0;
    const t = result.tokens;
    return (
      Object.keys(t.colors ?? {}).length +
      (t.typography?.length ?? 0) +
      (t.spacing?.length ?? 0) +
      (t.radii?.length ?? 0)
    );
  }, [result]);

  const componentCount = result?.inventory?.components?.length ?? 0;

  const onImport = useCallback(async () => {
    const key = resolveFileKey(input);
    if (!workspaceId) {
      setError('Open a workspace before importing from Figma.');
      return;
    }
    if (!key) {
      setError('Paste a Figma URL (figma.com/file/..., /design/..., /proto/...) or a raw file key.');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const r = await api.figmaImport(projectId, key, workspaceId);
      setResult(r);
      setTab('tokens');
      onImported?.({
        tokenCount:
          Object.keys(r.tokens?.colors ?? {}).length +
          (r.tokens?.typography?.length ?? 0) +
          (r.tokens?.spacing?.length ?? 0) +
          (r.tokens?.radii?.length ?? 0),
        componentCount: r.inventory?.components?.length ?? 0,
      });
    } catch (e) {
      setError(String((e as Error)?.message ?? e));
    } finally {
      setBusy(false);
    }
  }, [input, onImported, projectId, workspaceId]);

  return (
    <Box sx={{
      borderRadius: 1.6,
      border: `1px solid ${tokens.color.border.subtle}`,
      bgcolor: tokens.color.bg.surface,
      p: 1.4,
    }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
        <Palette fontSize="small" sx={{ color: tokens.color.accent.lime }} />
        <Typography variant="overline" sx={{ color: tokens.color.text.secondary, fontWeight: 800, letterSpacing: '0.1em' }}>
          Figma import
        </Typography>
      </Stack>

      <Typography variant="body2" sx={{ color: tokens.color.text.secondary, mb: 1.2 }}>
        Pull tokens and a frame inventory from a Figma file. The UX gate uses these as the source of truth.
      </Typography>

      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
        <TextField
          size="small"
          fullWidth
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="https://figma.com/file/ABC123def... or ABC123def..."
          disabled={busy}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && canSubmit) {
              e.preventDefault();
              void onImport();
            }
          }}
          InputProps={{ sx: { fontFamily: tokens.font.mono, fontSize: 12.5 } }}
        />
        <Button
          variant="contained"
          startIcon={busy ? <CircularProgress size={14} sx={{ color: tokens.color.text.inverse }} /> : <CloudDownload fontSize="small" />}
          onClick={() => void onImport()}
          disabled={!canSubmit}
          sx={{
            bgcolor: tokens.color.accent.lime,
            color: tokens.color.text.inverse,
            fontWeight: 800,
            '&:hover': { bgcolor: '#7dffd0' },
            '&.Mui-disabled': {
              bgcolor: 'rgba(226,236,248,0.08)',
              color: 'rgba(226,236,248,0.38)',
            },
            flexShrink: 0,
            minHeight: 36,
          }}
        >
          {busy ? 'Importing' : 'Import'}
        </Button>
      </Stack>

      <Box sx={{ mt: 0.8 }}>
        <Box
          component="details"
          sx={{
            '& > summary': {
              cursor: 'pointer',
              fontSize: 11.5,
              color: tokens.color.text.muted,
              fontWeight: 700,
              letterSpacing: '0.04em',
              listStyle: 'none',
              userSelect: 'none',
              '&::-webkit-details-marker': { display: 'none' },
              '&:hover': { color: tokens.color.text.primary },
            },
            '& > summary::before': {
              content: '"› "',
              display: 'inline-block',
              transition: 'transform 120ms',
            },
            '&[open] > summary::before': { transform: 'rotate(90deg)' },
          }}
        >
          <Box component="summary">Where do I find the file key?</Box>
          <Typography variant="caption" sx={{
            display: 'block', mt: 0.6,
            color: tokens.color.text.secondary,
            lineHeight: 1.5,
          }}>
            Open your Figma file, copy the URL from the browser — it looks like
            {' '}<Box component="code" sx={{
              fontFamily: tokens.font.mono,
              fontSize: 11,
              bgcolor: tokens.color.bg.inset,
              px: 0.6, py: 0.1, borderRadius: 0.6,
            }}>figma.com/file/ABC123def.../My-Design</Box>. Paste the whole URL
            or just the <Box component="code" sx={{
              fontFamily: tokens.font.mono,
              fontSize: 11,
              bgcolor: tokens.color.bg.inset,
              px: 0.6, py: 0.1, borderRadius: 0.6,
            }}>ABC123def...</Box> portion.
          </Typography>
        </Box>
      </Box>

      {error && (
        <Box sx={{
          mt: 1.2,
          px: 1.2, py: 0.9,
          borderRadius: 1.2,
          bgcolor: 'rgba(220,38,38,0.08)',
          border: '1px solid rgba(220,38,38,0.4)',
        }}>
          <Typography variant="caption" sx={{ color: tokens.color.accent.danger, fontWeight: 800, display: 'block' }}>
            Could not import
          </Typography>
          <Typography variant="caption" sx={{ color: tokens.color.text.secondary }}>
            {error}
          </Typography>
        </Box>
      )}

      {result && !error && (
        <Box sx={{ mt: 1.4 }}>
          <Box sx={{
            px: 1.2, py: 0.9, mb: 1,
            borderRadius: 1.2,
            bgcolor: 'rgba(83,255,189,0.08)',
            border: `1px solid ${tokens.color.border.accent}`,
          }}>
            <Typography variant="caption" sx={{ color: tokens.color.text.primary, fontWeight: 800, display: 'block' }}>
              {result.file?.name || 'Figma file'} imported
            </Typography>
            <Typography variant="caption" sx={{ color: tokens.color.text.secondary }}>
              {tokenCount} tokens · {componentCount} components
              {result.file?.lastModified
                ? ` · last modified ${new Date(result.file.lastModified).toLocaleDateString()}`
                : ''}
            </Typography>
          </Box>

          <Tabs
            value={tab}
            onChange={(_, v) => setTab(v as 'tokens' | 'frames')}
            sx={{
              minHeight: 34,
              borderBottom: `1px solid ${tokens.color.border.subtle}`,
              '& .MuiTab-root': {
                minHeight: 34, fontSize: 12, fontWeight: 800,
                color: tokens.color.text.muted, textTransform: 'none',
                px: 1.2,
              },
              '& .Mui-selected': { color: tokens.color.text.primary },
              '& .MuiTabs-indicator': { bgcolor: tokens.color.accent.lime, height: 2 },
            }}
          >
            <Tab value="tokens" label={`Tokens (${tokenCount})`} />
            <Tab value="frames" label={`Frames (${componentCount})`} />
          </Tabs>

          <Box sx={{ pt: 1.2 }}>
            {tab === 'tokens' && <TokensTab result={result} />}
            {tab === 'frames' && <FramesTab result={result} />}
          </Box>
        </Box>
      )}
    </Box>
  );
}

function TokensTab({ result }: { result: FigmaImportResult }) {
  const t = result.tokens;
  const colors = Object.entries(t.colors ?? {});
  const typography = t.typography ?? [];
  const spacing = t.spacing ?? [];
  const radii = t.radii ?? [];

  return (
    <Stack spacing={1.4}>
      <TokenSection title="Colors" count={colors.length}>
        {colors.length === 0 ? (
          <EmptyHint text="No color tokens detected." />
        ) : (
          <Box sx={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(120px, 1fr))',
            gap: 0.8,
          }}>
            {colors.map(([name, value]) => (
              <Box key={name} sx={{
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: 1,
                overflow: 'hidden',
                bgcolor: tokens.color.bg.inset,
              }}>
                <Box sx={{
                  height: 36,
                  bgcolor: value,
                  borderBottom: `1px solid ${tokens.color.border.subtle}`,
                }} />
                <Box sx={{ px: 0.7, py: 0.5 }}>
                  <Typography variant="caption" sx={{ display: 'block', fontWeight: 800, fontSize: 10.5 }} noWrap title={name}>
                    {name}
                  </Typography>
                  <Typography variant="caption" sx={{
                    display: 'block',
                    fontFamily: tokens.font.mono, fontSize: 10,
                    color: tokens.color.text.muted,
                  }} noWrap>
                    {value}
                  </Typography>
                </Box>
              </Box>
            ))}
          </Box>
        )}
      </TokenSection>

      <TokenSection title="Typography" count={typography.length}>
        {typography.length === 0 ? (
          <EmptyHint text="No typography tokens detected." />
        ) : (
          <Stack spacing={0.4}>
            {typography.map((ty, i) => {
              const label = describeTypography(ty);
              return (
                <Box key={i} sx={{
                  px: 0.8, py: 0.5,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  borderRadius: 0.8,
                  bgcolor: tokens.color.bg.inset,
                  fontFamily: tokens.font.mono, fontSize: 11,
                  color: tokens.color.text.secondary,
                }}>
                  {label}
                </Box>
              );
            })}
          </Stack>
        )}
      </TokenSection>

      <TokenSection title="Spacing" count={spacing.length}>
        {spacing.length === 0 ? (
          <EmptyHint text="No spacing tokens detected." />
        ) : (
          <Stack direction="row" spacing={0.6} sx={{ flexWrap: 'wrap' }} useFlexGap>
            {spacing.map((s, i) => (
              <Box key={i} sx={chipSx}>{s}px</Box>
            ))}
          </Stack>
        )}
      </TokenSection>

      <TokenSection title="Radii" count={radii.length}>
        {radii.length === 0 ? (
          <EmptyHint text="No radius tokens detected." />
        ) : (
          <Stack direction="row" spacing={0.6} sx={{ flexWrap: 'wrap' }} useFlexGap>
            {radii.map((r, i) => (
              <Box key={i} sx={chipSx}>{r}px</Box>
            ))}
          </Stack>
        )}
      </TokenSection>
    </Stack>
  );
}

function FramesTab({ result }: { result: FigmaImportResult }) {
  const components = result.inventory?.components ?? [];
  if (components.length === 0) {
    return <EmptyHint text="No frames or components found at the top level of the file." />;
  }
  return (
    <Box sx={{
      display: 'grid',
      gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))',
      gap: 0.8,
      maxHeight: 320,
      overflowY: 'auto',
      pr: 0.4,
    }}>
      {components.map((c) => (
        <Box key={c.nodeId} sx={{
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1.2,
          bgcolor: tokens.color.bg.inset,
          p: 0.9,
        }}>
          <Typography variant="caption" sx={{ display: 'block', fontWeight: 800 }} noWrap title={c.name}>
            {c.name || `Node ${c.nodeId}`}
          </Typography>
          <Typography variant="caption" sx={{
            display: 'block',
            fontFamily: tokens.font.mono, fontSize: 10.5,
            color: tokens.color.text.muted,
          }}>
            {Math.round(c.width)}×{Math.round(c.height)} · {c.type}
          </Typography>
        </Box>
      ))}
    </Box>
  );
}

function TokenSection({
  title, count, children,
}: { title: string; count: number; children: React.ReactNode }) {
  return (
    <Box>
      <Stack direction="row" alignItems="baseline" spacing={0.6} sx={{ mb: 0.6 }}>
        <Typography variant="overline" sx={{ color: tokens.color.text.secondary, fontWeight: 800, letterSpacing: '0.1em' }}>
          {title}
        </Typography>
        <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>
          {count}
        </Typography>
      </Stack>
      {children}
    </Box>
  );
}

function EmptyHint({ text }: { text: string }) {
  return (
    <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontStyle: 'italic' }}>
      {text}
    </Typography>
  );
}

function describeTypography(ty: FigmaTypographyToken): string {
  if (!ty || typeof ty !== 'object') return String(ty);
  const family = ty.fontFamily || ty.family || ty.font || '';
  const size = ty.fontSize ?? ty.size;
  const weight = ty.fontWeight ?? ty.weight;
  const name = ty.name || ty.id || '';
  const parts: string[] = [];
  if (name) parts.push(String(name));
  if (family) parts.push(String(family));
  if (typeof size === 'number') parts.push(`${size}px`);
  if (weight) parts.push(`w${weight}`);
  if (parts.length === 0) {
    try { return JSON.stringify(ty); } catch { return '[token]'; }
  }
  return parts.join(' · ');
}

const chipSx = {
  px: 0.9, py: 0.3,
  borderRadius: 0.8,
  border: `1px solid ${tokens.color.border.subtle}`,
  bgcolor: tokens.color.bg.inset,
  fontFamily: tokens.font.mono, fontSize: 11,
  color: tokens.color.text.secondary,
  fontWeight: 700,
} as const;
