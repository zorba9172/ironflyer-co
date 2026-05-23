'use client';

// PreviewPane — iframe with toolbar: refresh, open-in-new-tab, viewport size.
// URL resolution lives in lib/api/runtime-preview so backend changes don't
// leak into UI components.

import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Box, Button, Card, Chip, CircularProgress, Dialog, DialogActions, DialogContent, DialogTitle,
  IconButton, MenuItem, Select, Skeleton, Stack, TextField, Tooltip, Typography,
} from '@mui/material';
import {
  Close, DesktopWindows, EditLocation, FormatColorFill, Image as ImageIcon,
  IosShare, Laptop, MoreHoriz, OpenInNew, Refresh, Smartphone, TabletMac, TextFields, Tune,
} from '@mui/icons-material';
import { Workspace } from '../../lib/runtime';
import { PortMapping, getWorkspacePorts, mintShareLink, resolvePreviewURL } from '../../lib/api/runtime-preview';
import { api } from '../../lib/api';
import type { Patch } from '../../lib/api/patches';
import { tokens } from '../../lib/theme';

type Device = 'mobile' | 'tablet' | 'desktop';

// --- Visual Editor types ----------------------------------------------------
// QuickAction is the inline action menu the user pops at the click position.
// "custom" is the escape hatch that re-opens the original full Dialog so the
// half-version flow stays available as a fallback.
type QuickAction = 'text' | 'colour' | 'image' | 'custom';

// PendingEdit is a row in the small queue chip strip rendered above the
// iframe while editMode is active. Each one represents one visualEdit call
// the user already submitted from the floating action panel.
interface PendingEdit {
  id: string;
  label: string;
  status: 'pending' | 'applied' | 'failed';
  error?: string;
}

// ClickAnchor is the position (in the preview-box's own coord space) where
// the user clicked. The action panel positions itself there and the
// percentage form is what we send to the agent as a hint.
interface ClickAnchor {
  x: number;       // px from preview-box top-left
  y: number;       // px from preview-box top-left
  xPct: number;    // 0..100 of preview-box width
  yPct: number;    // 0..100 of preview-box height
}

const HOVER_BOX = 40;          // hover-highlight box size (px)
const HELP_LS_KEY = 'ironflyer.visualEditor.helpSeen.v1';
const IMAGE_INLINE_LIMIT = 200 * 1024; // 200 KB cap for data-URL inlining

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

  // --- Visual-editor (cleaner version) state -------------------------------
  // hoverPos is where the dashed bounding-box hover indicator follows the
  // cursor. null = cursor outside the overlay so we hide it.
  const [hoverPos, setHoverPos] = useState<{ x: number; y: number } | null>(null);
  // clickAnchor is the click point that anchors the floating action panel.
  // While set, the user is mid-action; clearing it dismisses the panel.
  const [clickAnchor, setClickAnchor] = useState<ClickAnchor | null>(null);
  // activeAction is which quick action the user picked from the panel.
  // null = they're still looking at the four buttons, not yet committed.
  const [activeAction, setActiveAction] = useState<QuickAction | null>(null);
  // inlineText is the buffer for the "Edit text" inline input.
  const [inlineText, setInlineText] = useState('');
  // pickedColour is the colour the user chose for "Change colour".
  const [pickedColour, setPickedColour] = useState('#e5ff00');
  // pendingEdits is the queue strip rendered above the iframe.
  const [pendingEdits, setPendingEdits] = useState<PendingEdit[]>([]);
  // showHelp is the one-shot first-enable tooltip; we persist dismissal.
  const [showHelp, setShowHelp] = useState(false);
  const overlayRef = useRef<HTMLDivElement | null>(null);
  const imageInputRef = useRef<HTMLInputElement | null>(null);

  // Dismiss the floating action panel on Escape so the user always has a
  // keyboard way out — the panel is a portal-less overlay and the iframe is
  // pointer-events:none under it so we don't lose focus to it.
  useEffect(() => {
    if (!clickAnchor) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setClickAnchor(null);
        setActiveAction(null);
        setInlineText('');
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [clickAnchor]);

  // First-enable help tooltip — shown once per browser, dismiss persists.
  useEffect(() => {
    if (!editMode) { setShowHelp(false); return; }
    if (typeof window === 'undefined') return;
    try {
      if (window.localStorage.getItem(HELP_LS_KEY)) return;
    } catch { /* private mode; just show it */ }
    setShowHelp(true);
    const t = window.setTimeout(() => setShowHelp(false), 6500);
    return () => window.clearTimeout(t);
  }, [editMode]);

  // submitVisualEdit pushes one PendingEdit onto the queue and fires the
  // existing api.visualEdit endpoint — same backend as the Dialog flow.
  // The chip updates from "pending" to "applied" / "failed" in place, and on
  // success we still call onPatchProposed so the PatchDrawer pops as before.
  const submitVisualEdit = async (
    label: string,
    body: Parameters<typeof api.visualEdit>[1],
  ) => {
    if (!projectId) return;
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
    setPendingEdits((q) => [...q, { id, label, status: 'pending' }]);
    try {
      const patch = await api.visualEdit(projectId, body);
      setPendingEdits((q) => q.map((p) => p.id === id ? { ...p, status: 'applied' } : p));
      if (onPatchProposed) onPatchProposed(patch);
    } catch (e) {
      const msg = String((e as Error)?.message ?? e);
      setPendingEdits((q) => q.map((p) => p.id === id ? { ...p, status: 'failed', error: msg } : p));
    }
  };

  // closeActionPanel resets every transient piece of state the floating
  // panel uses — we keep one helper so all the exits stay in sync.
  const closeActionPanel = () => {
    setClickAnchor(null);
    setActiveAction(null);
    setInlineText('');
  };

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
            // Lime border while in edit mode so it's unambiguous we've
            // hijacked the iframe's click target. Pixel-width stays the
            // same so layout doesn't shift when toggling.
            border: editMode
              ? `2px solid ${tokens.color.accent.lime}`
              : (device === 'desktop' ? `1px solid ${tokens.color.border.subtle}` : '6px solid #111'),
            transition: `border-color ${tokens.motion.fast} ${tokens.motion.curve}`,
          }}>
            <iframe
              key={`${url}-${bump}`}
              src={url}
              title="Live preview"
              sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals"
              style={{
                width: '100%', height: '100%', border: 0, display: 'block', background: '#fff',
                // Desaturate while editing so the chrome reads as "edit mode"
                // without us touching the iframe DOM (cross-origin: forbidden).
                filter: editMode ? 'saturate(0.7) brightness(0.95)' : 'none',
                transition: `filter ${tokens.motion.base} ${tokens.motion.curve}`,
                pointerEvents: editMode ? 'none' : 'auto',
              }}
            />
            {editMode && (
              <Box
                ref={overlayRef}
                onMouseMove={(e) => {
                  const rect = (e.currentTarget as HTMLDivElement).getBoundingClientRect();
                  setHoverPos({ x: e.clientX - rect.left, y: e.clientY - rect.top });
                }}
                onMouseLeave={() => setHoverPos(null)}
                onClick={(e) => {
                  // If the action panel is already open, treat clicks
                  // outside the panel itself as "dismiss" so the user can
                  // re-pick a location without an explicit close button.
                  if (clickAnchor) { closeActionPanel(); return; }
                  const rect = (e.currentTarget as HTMLDivElement).getBoundingClientRect();
                  const x = e.clientX - rect.left;
                  const y = e.clientY - rect.top;
                  const xPct = (x / rect.width) * 100;
                  const yPct = (y / rect.height) * 100;
                  setClickAnchor({ x, y, xPct, yPct });
                  setActiveAction(null);
                  setInlineText('');
                }}
                sx={{
                  position: 'absolute', inset: 0, cursor: 'crosshair',
                  bgcolor: 'rgba(170, 230, 90, 0.06)',
                  outline: `2px dashed ${tokens.color.accent.lime}`,
                  outlineOffset: '-6px',
                }}
              >
                {/* Hover-highlight bounding box. We can't read iframe DOM
                    (cross-origin), so we draw a constant-size box centred
                    on the cursor — the agent treats it as a "near here"
                    hint, not a literal selector. */}
                {hoverPos && !clickAnchor && (
                  <Box
                    aria-hidden
                    sx={{
                      position: 'absolute',
                      left: hoverPos.x - HOVER_BOX / 2,
                      top: hoverPos.y - HOVER_BOX / 2,
                      width: HOVER_BOX,
                      height: HOVER_BOX,
                      border: `4px dashed ${tokens.color.accent.lime}`,
                      borderRadius: '4px',
                      pointerEvents: 'none',
                      boxShadow: '0 0 0 1px rgba(0,0,0,0.35)',
                    }}
                  />
                )}

                {/* The persistent "selection box" at the click point — it
                    stays visible while the floating action panel is open so
                    the user can see what they targeted. */}
                {clickAnchor && (
                  <Box
                    aria-hidden
                    sx={{
                      position: 'absolute',
                      left: clickAnchor.x - HOVER_BOX / 2,
                      top: clickAnchor.y - HOVER_BOX / 2,
                      width: HOVER_BOX,
                      height: HOVER_BOX,
                      border: `4px solid ${tokens.color.accent.lime}`,
                      borderRadius: '4px',
                      pointerEvents: 'none',
                      boxShadow: '0 0 0 1px rgba(0,0,0,0.35)',
                    }}
                  />
                )}
              </Box>
            )}

            {/* Floating action panel — sits inside the preview box so its
                coords stay anchored to the click point even when the page
                scrolls. Stops click-propagation so it doesn't auto-dismiss. */}
            {editMode && clickAnchor && (
              <VisualEditActionPanel
                anchor={clickAnchor}
                containerWidth={overlayRef.current?.clientWidth ?? 0}
                containerHeight={overlayRef.current?.clientHeight ?? 0}
                active={activeAction}
                inlineText={inlineText}
                pickedColour={pickedColour}
                onPickAction={(a) => {
                  if (a === 'custom') {
                    // Hand off to the existing Dialog flow (the original
                    // half-version) — that's the fallback for anything the
                    // four quick actions don't cover.
                    setEditTarget({ xPct: clickAnchor.xPct, yPct: clickAnchor.yPct });
                    setEditSelector('');
                    setEditInstruction('');
                    setEditError(null);
                    closeActionPanel();
                    return;
                  }
                  if (a === 'image') {
                    setActiveAction('image');
                    // Defer file-open one tick so the input is mounted
                    // before we call .click().
                    window.setTimeout(() => imageInputRef.current?.click(), 0);
                    return;
                  }
                  setActiveAction(a);
                }}
                onSetText={setInlineText}
                onSetColour={setPickedColour}
                onSubmitText={() => {
                  if (!inlineText.trim()) return;
                  const sel = `the text at ${clickAnchor.xPct.toFixed(1)}%,${clickAnchor.yPct.toFixed(1)}%`;
                  submitVisualEdit(
                    `Text · "${truncate(inlineText.trim(), 28)}"`,
                    { selector: sel, instruction: `Replace text with: ${inlineText.trim()}` },
                  );
                  closeActionPanel();
                }}
                onSubmitColour={() => {
                  const sel = `the element at ${clickAnchor.xPct.toFixed(1)}%,${clickAnchor.yPct.toFixed(1)}%`;
                  submitVisualEdit(
                    `Colour · ${pickedColour}`,
                    { selector: sel, instruction: `Change the colour at this position to ${pickedColour}` },
                  );
                  closeActionPanel();
                }}
                onClose={closeActionPanel}
              />
            )}

            {/* Hidden file input — wired to the "Replace with image" action.
                Reuses ChatPane's FileReader → base64 pattern so we don't
                pull a new dep. Anything over 200KB falls back to a
                description-only instruction (the inline data URL would
                blow up the prompt budget otherwise). */}
            {editMode && (
              <input
                ref={imageInputRef}
                type="file"
                accept="image/png,image/jpeg,image/webp,image/gif"
                style={{ display: 'none' }}
                onChange={async (e) => {
                  const file = e.target.files?.[0];
                  e.target.value = '';
                  if (!file || !clickAnchor) return;
                  const sel = `the image at ${clickAnchor.xPct.toFixed(1)}%,${clickAnchor.yPct.toFixed(1)}%`;
                  if (file.size <= IMAGE_INLINE_LIMIT) {
                    const dataUrl = await readImageAsDataURL(file);
                    submitVisualEdit(
                      `Image · ${truncate(file.name, 22)}`,
                      {
                        selector: sel,
                        instruction:
                          `Replace the image at this position with the attached image (filename: ${file.name}). ` +
                          `Inline data URL: ${dataUrl}`,
                      },
                    );
                  } else {
                    submitVisualEdit(
                      `Image · ${truncate(file.name, 22)} (large)`,
                      {
                        selector: sel,
                        instruction:
                          `Replace the image at this position. The user picked a local file "${file.name}" ` +
                          `(${Math.round(file.size / 1024)} KB) — generate or source an equivalent image and wire it in.`,
                      },
                    );
                  }
                  closeActionPanel();
                }}
              />
            )}

            {/* Pending edits chip strip — sits at the top of the preview
                while editMode is on, so the user can watch each
                visualEdit land without leaving the preview pane. */}
            {editMode && pendingEdits.length > 0 && (
              <Box sx={{
                position: 'absolute', top: 8, left: 8, right: 8,
                display: 'flex', flexWrap: 'wrap', gap: 0.6,
                pointerEvents: 'none',
              }}>
                <Chip
                  size="small"
                  label={`${pendingEdits.length} edit${pendingEdits.length === 1 ? '' : 's'}`}
                  sx={{
                    bgcolor: tokens.color.bg.surface,
                    color: tokens.color.text.primary,
                    fontWeight: 700,
                    border: `1px solid ${tokens.color.border.subtle}`,
                    pointerEvents: 'auto',
                  }}
                />
                {pendingEdits.slice(-5).map((p) => (
                  <Tooltip key={p.id} title={p.error ?? p.label}>
                    <Chip
                      size="small"
                      label={p.label}
                      icon={p.status === 'pending'
                        ? <CircularProgress size={10} sx={{ color: tokens.color.text.inverse }} />
                        : undefined}
                      sx={{
                        pointerEvents: 'auto',
                        fontWeight: 600,
                        bgcolor:
                          p.status === 'applied' ? tokens.color.accent.success :
                          p.status === 'failed'  ? tokens.color.accent.danger :
                          tokens.color.accent.lime,
                        color: tokens.color.text.inverse,
                        maxWidth: 220,
                        '& .MuiChip-label': { textOverflow: 'ellipsis', overflow: 'hidden' },
                      }}
                      onDelete={p.status !== 'pending'
                        ? () => setPendingEdits((q) => q.filter((x) => x.id !== p.id))
                        : undefined}
                      deleteIcon={<Close sx={{ fontSize: 14 }} />}
                    />
                  </Tooltip>
                ))}
              </Box>
            )}

            {/* First-enable help tooltip. Anchored to the iframe so it
                disappears with the preview if the workspace goes away. */}
            {editMode && showHelp && (
              <Box
                onClick={() => {
                  setShowHelp(false);
                  try { window.localStorage.setItem(HELP_LS_KEY, '1'); } catch { /* ignore */ }
                }}
                sx={{
                  position: 'absolute', bottom: 12, left: '50%',
                  transform: 'translateX(-50%)',
                  bgcolor: tokens.color.bg.surface,
                  border: `1px solid ${tokens.color.accent.lime}`,
                  color: tokens.color.text.primary,
                  borderRadius: tokens.radius.md + 'px',
                  px: 1.6, py: 0.9,
                  fontSize: 12, fontFamily: tokens.font.mono,
                  boxShadow: tokens.shadow.md,
                  cursor: 'pointer',
                  maxWidth: 360,
                }}
              >
                Click any element to edit it. Use “Custom edit…” for anything advanced.
              </Box>
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

// VisualEditActionPanel is the floating Card with the four quick actions.
// It positions itself near the click point, but clamps to the container so
// it never escapes the preview viewport. Clicks inside don't propagate to
// the overlay (which would dismiss the panel).
function VisualEditActionPanel({
  anchor, containerWidth, containerHeight,
  active, inlineText, pickedColour,
  onPickAction, onSetText, onSetColour, onSubmitText, onSubmitColour, onClose,
}: {
  anchor: ClickAnchor;
  containerWidth: number;
  containerHeight: number;
  active: QuickAction | null;
  inlineText: string;
  pickedColour: string;
  onPickAction: (a: QuickAction) => void;
  onSetText: (v: string) => void;
  onSetColour: (v: string) => void;
  onSubmitText: () => void;
  onSubmitColour: () => void;
  onClose: () => void;
}) {
  // Panel size estimate — we clamp using these so the panel never spills
  // outside the preview. The text/colour expanded panes are wider than the
  // base four-button grid so we measure off whichever variant is active.
  const PANEL_W = active === 'text' ? 280 : active === 'colour' ? 240 : 220;
  const PANEL_H = active === 'text' ? 130 : active === 'colour' ? 140 : 132;
  const margin = 8;

  // Default: panel sits just below-right of the click. Flip sides if we'd
  // overflow either edge. containerWidth/Height may be 0 on first paint
  // (the ref-callback hasn't measured yet) so fall back to "below-right".
  let left = anchor.x + 14;
  let top  = anchor.y + 14;
  if (containerWidth  && left + PANEL_W + margin > containerWidth)  left = anchor.x - PANEL_W - 14;
  if (containerHeight && top  + PANEL_H + margin > containerHeight) top  = anchor.y - PANEL_H - 14;
  if (left < margin) left = margin;
  if (top  < margin) top  = margin;

  return (
    <Card
      role="dialog"
      aria-label="Visual edit actions"
      onClick={(e) => e.stopPropagation()}
      sx={{
        position: 'absolute',
        left, top,
        width: PANEL_W,
        zIndex: 5,
        p: 1.1,
        bgcolor: tokens.color.bg.surfaceRaised,
        border: `1px solid ${tokens.color.accent.lime}`,
        borderRadius: tokens.radius.md + 'px',
        boxShadow: tokens.shadow.lg,
      }}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 0.6 }}>
        <Typography variant="caption" sx={{
          color: tokens.color.text.muted, fontFamily: tokens.font.mono,
        }}>
          {anchor.xPct.toFixed(0)}% / {anchor.yPct.toFixed(0)}%
        </Typography>
        <IconButton size="small" onClick={onClose} sx={{ p: 0.3, color: tokens.color.text.muted }}>
          <Close sx={{ fontSize: 14 }} />
        </IconButton>
      </Stack>

      {/* Base four-action grid. Switches to the expanded pane once a
          mode-with-input ("text" / "colour") is selected. */}
      {active === null && (
        <Stack spacing={0.6}>
          <ActionRow icon={<TextFields fontSize="small" />} label="Edit text"          onClick={() => onPickAction('text')}   />
          <ActionRow icon={<FormatColorFill fontSize="small" />} label="Change colour" onClick={() => onPickAction('colour')} />
          <ActionRow icon={<ImageIcon fontSize="small" />} label="Replace with image"  onClick={() => onPickAction('image')}  />
          <ActionRow icon={<Tune fontSize="small" />} label="Custom edit…"             onClick={() => onPickAction('custom')} />
        </Stack>
      )}

      {active === 'text' && (
        <Stack spacing={0.8}>
          <TextField
            autoFocus
            size="small"
            value={inlineText}
            placeholder="New text…"
            onChange={(e) => onSetText(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && inlineText.trim()) onSubmitText(); }}
            fullWidth
          />
          <Stack direction="row" spacing={0.6} justifyContent="flex-end">
            <Button size="small" onClick={onClose}>Cancel</Button>
            <Button
              size="small" variant="contained"
              disabled={!inlineText.trim()}
              onClick={onSubmitText}
            >
              Apply
            </Button>
          </Stack>
        </Stack>
      )}

      {active === 'colour' && (
        <Stack spacing={0.8}>
          <Stack direction="row" alignItems="center" spacing={1}>
            <Box
              component="input"
              type="color"
              value={pickedColour}
              onChange={(e) => onSetColour((e.target as HTMLInputElement).value)}
              sx={{
                width: 38, height: 38,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: tokens.radius.sm + 'px',
                bgcolor: tokens.color.bg.inset,
                cursor: 'pointer',
                p: 0,
              }}
            />
            <TextField
              size="small"
              value={pickedColour}
              onChange={(e) => onSetColour(e.target.value)}
              sx={{ flex: 1, '& input': { fontFamily: tokens.font.mono } }}
            />
          </Stack>
          <Stack direction="row" spacing={0.6} justifyContent="flex-end">
            <Button size="small" onClick={onClose}>Cancel</Button>
            <Button size="small" variant="contained" onClick={onSubmitColour}>
              Apply
            </Button>
          </Stack>
        </Stack>
      )}

      {active === 'image' && (
        <Stack spacing={0.6} alignItems="center" sx={{ py: 1 }}>
          <MoreHoriz sx={{ color: tokens.color.text.muted }} />
          <Typography variant="caption" sx={{ color: tokens.color.text.muted }}>
            Pick an image…
          </Typography>
        </Stack>
      )}
    </Card>
  );
}

function ActionRow({ icon, label, onClick }: { icon: React.ReactNode; label: string; onClick: () => void }) {
  return (
    <Button
      onClick={onClick}
      startIcon={icon}
      sx={{
        justifyContent: 'flex-start',
        textTransform: 'none',
        color: tokens.color.text.primary,
        bgcolor: tokens.color.bg.inset,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: tokens.radius.sm + 'px',
        py: 0.6, px: 1,
        fontSize: 13, fontWeight: 600,
        '&:hover': {
          bgcolor: tokens.color.bg.surfaceHover,
          borderColor: tokens.color.border.accent,
          transform: 'none',
        },
      }}
    >
      {label}
    </Button>
  );
}

// readImageAsDataURL is a small mirror of ChatPane's helper — kept inline
// so we don't reach across components for one util. Returns the full
// `data:image/...;base64,...` URL because visual-edit instructions are
// free text on the wire (not a typed image attachment).
function readImageAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onerror = () => reject(r.error ?? new Error('FileReader error'));
    r.onload = () => resolve(String(r.result || ''));
    r.readAsDataURL(file);
  });
}

function truncate(s: string, n: number): string {
  return s.length <= n ? s : s.slice(0, n - 1) + '…';
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
