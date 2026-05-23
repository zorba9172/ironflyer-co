'use client';

// PreviewPane — iframe with toolbar: refresh, open-in-new-tab, viewport size.
// URL resolution lives in lib/api/runtime-preview so backend changes don't
// leak into UI components.

import { useEffect, useMemo, useState } from 'react';
import {
  Box, Button, Dialog, DialogActions, DialogContent, DialogTitle,
  IconButton, MenuItem, Select, Skeleton, Stack, TextField, Tooltip, Typography,
} from '@mui/material';
import {
  DesktopWindows, EditLocation, IosShare, Laptop, OpenInNew, Refresh, Smartphone, TabletMac,
} from '@mui/icons-material';
import { Workspace } from '../../lib/runtime';
import { PortMapping, getWorkspacePorts, mintShareLink, resolvePreviewURL } from '../../lib/api/runtime-preview';
import { api } from '../../lib/api';
import type { Patch } from '../../lib/api/patches';
import { tokens } from '../../lib/theme';

type Device = 'mobile' | 'tablet' | 'desktop';

interface Props {
  workspace: Workspace | null;
  // when the run completes, the parent bumps this to force a soft refresh
  refreshKey?: number;
  // projectId enables the click-to-edit flow. Without it the Edit button
  // is hidden — the preview falls back to read-only iframe rendering.
  projectId?: string;
  // onPatchProposed lets the parent (the project workspace shell) bump
  // its patches list / open the PatchDrawer once a visual-edit landed.
  onPatchProposed?: (patch: Patch) => void;
}

const DEVICE_SIZES: Record<Device, { w: number; h: number; label: string }> = {
  mobile:  { w: 390,  h: 780,  label: 'Mobile · 390' },
  tablet:  { w: 820,  h: 1180, label: 'Tablet · 820' },
  desktop: { w: 1280, h: 800,  label: 'Desktop · 1280' },
};

export function PreviewPane({ workspace, refreshKey = 0, projectId, onPatchProposed }: Props) {
  const [device, setDevice] = useState<Device>('desktop');
  const [ports, setPorts] = useState<PortMapping[]>([]);
  const [token, setToken] = useState<string | undefined>(undefined);
  const [selectedPort, setSelectedPort] = useState<number | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [bump, setBump] = useState(0);
  const [shareBusy, setShareBusy] = useState(false);
  const [shareMsg, setShareMsg] = useState<string | null>(null);
  const [editMode, setEditMode] = useState(false);
  const [editTarget, setEditTarget] = useState<{ xPct: number; yPct: number } | null>(null);
  const [editSelector, setEditSelector] = useState('');
  const [editInstruction, setEditInstruction] = useState('');
  const [editBusy, setEditBusy] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  useEffect(() => {
    if (!workspace) {
      setPorts([]); setToken(undefined); setError(null); setLoading(false);
      return;
    }
    let alive = true;
    setLoading(true);
    setError(null);
    const load = (showSpinner: boolean) => {
      if (showSpinner) setLoading(true);
      getWorkspacePorts(workspace.id)
        .then((res) => {
          if (!alive) return;
          setPorts(res.ports);
          setToken(res.previewToken);
          if (!selectedPort && res.ports[0]) setSelectedPort(res.ports[0].port);
          setError(null);
        })
        .catch((e) => {
          if (!alive) return;
          setError(String(e?.message ?? e));
        })
        .finally(() => alive && setLoading(false));
    };
    load(true);
    // Background re-poll so the health dot reflects current server state
    // without the user clicking Refresh. 6s matches the runtime's probe
    // budget (concurrent probes complete in ~1.5s, so we never queue up).
    const id = window.setInterval(() => load(false), 6000);
    return () => { alive = false; window.clearInterval(id); };
  }, [workspace?.id, refreshKey]);

  const url = useMemo(
    () => resolvePreviewURL(workspace, ports, token, selectedPort),
    [workspace, ports, token, selectedPort],
  );

  const size = DEVICE_SIZES[device];

  if (!workspace) {
    return (
      <EmptyShell
        title="No runtime yet"
        body="Run the Finisher or open Terminal to start a workspace. The preview will load here as soon as the server is up."
      />
    );
  }

  return (
    <Stack spacing={1.2} sx={{ height: '100%', minHeight: 0 }}>
      <Stack
        direction="row" alignItems="center" spacing={1}
        sx={{ flexWrap: 'wrap' }}
      >
        <DeviceToggle device={device} onChange={setDevice} />
        <Box sx={{ flex: 1, minWidth: 120 }}>
          {ports.length > 0 ? (
            <Select
              size="small"
              value={selectedPort ?? ''}
              onChange={(e) => setSelectedPort(Number(e.target.value))}
              sx={{
                minWidth: 140,
                bgcolor: tokens.color.bg.inset,
                fontFamily: tokens.font.mono, fontSize: 12,
                '& .MuiSelect-select': { py: 0.7 },
              }}
            >
              {ports.map((p) => (
                <MenuItem key={p.port} value={p.port}>
                  <Box component="span" sx={{
                    display: 'inline-block', width: 8, height: 8, mr: 1,
                    borderRadius: '50%', verticalAlign: 'middle',
                    bgcolor: portDotColor(p),
                  }} />
                  :{p.port} {portStatusLabel(p)}
                </MenuItem>
              ))}
            </Select>
          ) : (
            <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>
              {workspace.previewUrl ? `legacy preview` : 'awaiting a forwarded port'}
            </Typography>
          )}
        </Box>
        {projectId && (
          <Tooltip title={editMode ? 'Click an element to edit it' : 'Visual edit — click an element to instruct the agent'}>
            <span>
              <IconButton
                size="small"
                disabled={!url}
                onClick={() => setEditMode((m) => !m)}
                sx={{
                  color: editMode ? tokens.color.text.inverse : tokens.color.text.primary,
                  bgcolor: editMode ? tokens.color.accent.lime : 'transparent',
                  '&:hover': { bgcolor: editMode ? tokens.color.accent.lime : tokens.color.bg.surfaceHover },
                }}
              >
                <EditLocation fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
        )}
        <Tooltip title="Refresh preview">
          <span>
            <IconButton size="small" disabled={!url} onClick={() => setBump((b) => b + 1)}>
              <Refresh fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
        <Tooltip title={shareMsg ?? 'Copy a 7-day public share link to this preview'}>
          <span>
            <IconButton
              size="small"
              disabled={!workspace || !selectedPort || shareBusy}
              onClick={async () => {
                if (!workspace || !selectedPort) return;
                setShareBusy(true);
                setShareMsg(null);
                try {
                  const link = await mintShareLink(workspace.id, selectedPort);
                  if (!link) {
                    setShareMsg('Could not mint a link');
                    return;
                  }
                  if (typeof navigator !== 'undefined' && navigator.clipboard) {
                    try { await navigator.clipboard.writeText(link.url); } catch { /* ignore */ }
                  }
                  setShareMsg(`Copied — expires ${new Date(link.expiresAt).toLocaleDateString()}`);
                  window.setTimeout(() => setShareMsg(null), 6000);
                } finally {
                  setShareBusy(false);
                }
              }}
            >
              <IosShare fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
        <Tooltip title="Open in new tab">
          <span>
            <IconButton
              size="small" disabled={!url}
              component="a" href={url ?? '#'} target="_blank" rel="noopener noreferrer"
            >
              <OpenInNew fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      </Stack>

      <Box sx={{
        flex: 1, minHeight: 0,
        borderRadius: '12px',
        border: '1px solid rgba(17,17,17,0.12)',
        bgcolor: '#0d0e0f',
        overflow: 'auto',
        display: 'grid',
        placeItems: 'center',
        p: 1.4,
      }}>
        {loading ? (
          <Skeleton variant="rounded" sx={{ width: '90%', height: '90%' }} />
        ) : error ? (
          <ErrorState message={error} onRetry={() => setBump((b) => b + 1)} />
        ) : !url ? (
          <EmptyShell
            title="Preview will appear here"
            body="As soon as the agent starts a preview server, it will appear live here with hot reload."
            inset
          />
        ) : (
          <Box sx={{
            position: 'relative',
            width: device === 'desktop' ? '100%' : size.w,
            maxWidth: '100%',
            height: device === 'desktop' ? '100%' : Math.min(size.h, 900),
            borderRadius: device === 'desktop' ? '8px' : device === 'tablet' ? '20px' : '28px',
            overflow: 'hidden',
            bgcolor: '#ffffff',
            boxShadow: '0 18px 60px rgba(0,0,0,0.4)',
            border: device === 'desktop' ? `1px solid ${tokens.color.border.subtle}` : '6px solid #111',
          }}>
            <iframe
              key={`${url}-${bump}`}
              src={url}
              title="Live preview"
              sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals"
              style={{
                width: '100%', height: '100%', border: 0, display: 'block', background: '#fff',
                pointerEvents: editMode ? 'none' : 'auto',
              }}
            />
            {editMode && (
              <Box
                onClick={(e) => {
                  const rect = (e.currentTarget as HTMLDivElement).getBoundingClientRect();
                  const xPct = ((e.clientX - rect.left) / rect.width) * 100;
                  const yPct = ((e.clientY - rect.top) / rect.height) * 100;
                  setEditTarget({ xPct, yPct });
                  setEditSelector('');
                  setEditInstruction('');
                  setEditError(null);
                }}
                sx={{
                  position: 'absolute', inset: 0, cursor: 'crosshair',
                  bgcolor: 'rgba(170, 230, 90, 0.08)',
                  outline: `2px dashed ${tokens.color.accent.lime}`,
                  outlineOffset: '-6px',
                }}
              />
            )}
          </Box>
        )}
      </Box>
      <Dialog
        open={Boolean(editTarget)}
        onClose={() => !editBusy && setEditTarget(null)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>
          Visual edit
          {editTarget && (
            <Typography variant="caption" sx={{ display: 'block', color: tokens.color.text.muted }}>
              click at {editTarget.xPct.toFixed(1)}% / {editTarget.yPct.toFixed(1)}% of the viewport
            </Typography>
          )}
        </DialogTitle>
        <DialogContent>
          <Stack spacing={1.4} sx={{ pt: 0.5 }}>
            <TextField
              label="Selector or describe the element"
              placeholder="e.g. .cta-primary, the lime button in the hero, Section #pricing → first card"
              value={editSelector}
              onChange={(e) => setEditSelector(e.target.value)}
              autoFocus
              fullWidth
              helperText="Free-text is fine — the Coder reads it as a hint, not a literal query."
            />
            <TextField
              label="What should change?"
              placeholder="Tighten the padding, use the lime accent, change copy to 'Get started'"
              value={editInstruction}
              onChange={(e) => setEditInstruction(e.target.value)}
              multiline minRows={2} maxRows={6}
              fullWidth
            />
            {editError && (
              <Typography variant="caption" sx={{ color: tokens.color.accent.danger }}>
                {editError}
              </Typography>
            )}
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button disabled={editBusy} onClick={() => setEditTarget(null)}>Cancel</Button>
          <Button
            variant="contained"
            disabled={editBusy || !editSelector.trim() || !editInstruction.trim() || !projectId}
            onClick={async () => {
              if (!projectId) return;
              setEditBusy(true);
              setEditError(null);
              try {
                const patch = await api.visualEdit(projectId, {
                  selector: editSelector.trim(),
                  instruction: editInstruction.trim(),
                });
                setEditTarget(null);
                setEditMode(false);
                if (onPatchProposed) onPatchProposed(patch);
              } catch (e) {
                setEditError(String((e as Error)?.message ?? e));
              } finally {
                setEditBusy(false);
              }
            }}
          >
            {editBusy ? 'Asking the Coder…' : 'Propose patch'}
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  );
}

function DeviceToggle({ device, onChange }: { device: Device; onChange: (d: Device) => void }) {
  return (
    <Stack direction="row" spacing={0.3} sx={{
      p: 0.3, borderRadius: '10px',
      bgcolor: tokens.color.bg.inset, border: `1px solid ${tokens.color.border.subtle}`,
    }}>
      <ToggleBtn active={device === 'mobile'} onClick={() => onChange('mobile')} label="Mobile">
        <Smartphone fontSize="small" />
      </ToggleBtn>
      <ToggleBtn active={device === 'tablet'} onClick={() => onChange('tablet')} label="Tablet">
        <TabletMac fontSize="small" />
      </ToggleBtn>
      <ToggleBtn active={device === 'desktop'} onClick={() => onChange('desktop')} label="Desktop">
        <Laptop fontSize="small" />
      </ToggleBtn>
    </Stack>
  );
}

function ToggleBtn({
  active, onClick, label, children,
}: { active: boolean; onClick: () => void; label: string; children: React.ReactNode }) {
  return (
    <Tooltip title={label}>
      <IconButton
        size="small"
        onClick={onClick}
        sx={{
          width: 30, height: 28, borderRadius: '8px',
          color: active ? tokens.color.text.inverse : tokens.color.text.muted,
          bgcolor: active ? tokens.color.accent.lime : 'transparent',
          '&:hover': { bgcolor: active ? tokens.color.accent.lime : tokens.color.bg.surfaceHover },
        }}
      >
        {children}
      </IconButton>
    </Tooltip>
  );
}

function EmptyShell({ title, body, inset }: { title: string; body: string; inset?: boolean }) {
  return (
    <Stack spacing={1.2} alignItems="center" sx={{
      textAlign: 'center', px: 3, py: inset ? 0 : 6, maxWidth: 360,
      color: tokens.color.text.primary,
    }}>
      <Box sx={{
        width: 52, height: 52, borderRadius: '50%',
        display: 'grid', placeItems: 'center',
        bgcolor: tokens.color.bg.inset,
        color: tokens.color.accent.lime,
      }}>
        <DesktopWindows fontSize="small" />
      </Box>
      <Typography variant="subtitle1" sx={{ fontWeight: 800, color: tokens.color.text.primary }}>
        {title}
      </Typography>
      <Typography variant="body2" sx={{ color: tokens.color.text.muted }}>
        {body}
      </Typography>
    </Stack>
  );
}

function portDotColor(p: PortMapping): string {
  if (!p.ready) return tokens.color.text.muted;
  if (p.healthy === true) return tokens.color.accent.success;
  if (p.healthy === false) return tokens.color.accent.danger;
  return tokens.color.accent.lime; // probe unknown — allowed but unprobed
}

function portStatusLabel(p: PortMapping): string {
  if (!p.ready) return '· blocked';
  if (p.healthy === true) {
    return typeof p.latencyMs === 'number' ? `· live · ${p.latencyMs}ms` : '· live';
  }
  if (p.healthy === false) return '· starting…';
  return '· ready';
}

function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <Stack spacing={1.2} alignItems="center" sx={{ textAlign: 'center', px: 3, maxWidth: 360 }}>
      <Typography variant="subtitle1" sx={{ fontWeight: 800, color: tokens.color.accent.danger }}>
        We could not load the preview
      </Typography>
      <Typography variant="caption" sx={{ color: tokens.color.text.muted, maxWidth: 320 }} title={message}>
        {message.slice(0, 220)}
      </Typography>
      <Button variant="contained" size="small" onClick={onRetry}>Try again</Button>
    </Stack>
  );
}
