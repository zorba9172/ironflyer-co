'use client';

// VisualTargetsPanel — surfaces pixel-perfect references in the workspace
// sidebar. Each target is a PNG the UX gate diffs against the live preview
// at a fixed viewport. Uploading more targets makes the gate stricter —
// the run won't pass until the preview matches within tolerance.
//
// Storage is base64-on-the-wire; the orchestrator caps individual uploads
// at 4 MiB, so we mirror that limit client-side and refuse oversized files
// before the network round-trip.

import { useCallback, useEffect, useState } from 'react';
import {
  Box, Button, Dialog, IconButton, Skeleton, Slider, Stack, TextField,
  Tooltip, Typography,
} from '@mui/material';
import {
  Add as AddIcon, Close as CloseIcon, DeleteOutline,
  ImageOutlined,
} from '@mui/icons-material';

import { api, VisualTarget } from '../../lib/api';
import { tokens } from '../../lib/theme';

interface Props {
  projectId: string;
}

const MAX_BYTES = 4 * 1024 * 1024;

interface FormState {
  name: string;
  routeHint: string;
  viewportW: number;
  viewportH: number;
  tolerance: number;
  base64: string;
  previewDataUrl: string;
  fileName: string;
  fileBytes: number;
}

const EMPTY_FORM: FormState = {
  name: '',
  routeHint: '/',
  viewportW: 1280,
  viewportH: 800,
  tolerance: 0.02,
  base64: '',
  previewDataUrl: '',
  fileName: '',
  fileBytes: 0,
};

export function VisualTargetsPanel({ projectId }: Props) {
  const [items, setItems] = useState<VisualTarget[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [adding, setAdding] = useState(false);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [showRerunHint, setShowRerunHint] = useState(false);

  const [zoomTarget, setZoomTarget] = useState<VisualTarget | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await api.listVisualTargets(projectId));
    } catch (e) {
      setError(String((e as Error)?.message ?? e));
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => { void load(); }, [load]);

  const onPickFile = useCallback(async (file: File) => {
    setFormError(null);
    if (file.size > MAX_BYTES) {
      setFormError(`Image must be ≤ 4 MiB (got ${(file.size / (1024 * 1024)).toFixed(2)} MiB).`);
      return;
    }
    try {
      const { dataUrl, base64 } = await readImageAsBase64(file);
      setForm((f) => ({
        ...f,
        base64,
        previewDataUrl: dataUrl,
        fileName: file.name,
        fileBytes: file.size,
        name: f.name || file.name.replace(/\.[^.]+$/, ''),
      }));
    } catch (e) {
      setFormError(String((e as Error)?.message ?? e));
    }
  }, []);

  const onSubmit = useCallback(async () => {
    if (!form.base64) {
      setFormError('Choose a PNG to upload.');
      return;
    }
    if (form.viewportW <= 0 || form.viewportH <= 0) {
      setFormError('Viewport width and height must be positive.');
      return;
    }
    setSubmitting(true);
    setFormError(null);
    try {
      await api.addVisualTarget(projectId, {
        name: form.name.trim() || undefined,
        routeHint: form.routeHint.trim() || undefined,
        viewportW: form.viewportW,
        viewportH: form.viewportH,
        tolerance: form.tolerance,
        imagePngBase64: form.base64,
      });
      setForm(EMPTY_FORM);
      setAdding(false);
      setShowRerunHint(true);
      await load();
    } catch (e) {
      setFormError(String((e as Error)?.message ?? e));
    } finally {
      setSubmitting(false);
    }
  }, [form, load, projectId]);

  const onDelete = useCallback(async (targetId: string) => {
    setBusyId(targetId);
    try {
      await api.deleteVisualTarget(projectId, targetId);
      await load();
    } catch (e) {
      setError(String((e as Error)?.message ?? e));
    } finally {
      setBusyId(null);
    }
  }, [load, projectId]);

  return (
    <Stack spacing={1.2}>
      {!adding && (
        <Button
          size="small"
          startIcon={<AddIcon />}
          onClick={() => { setAdding(true); setFormError(null); setShowRerunHint(false); }}
          sx={{
            alignSelf: 'flex-start',
            bgcolor: tokens.color.accent.lime,
            color: tokens.color.text.inverse,
            fontWeight: 800,
            '&:hover': { bgcolor: '#f0ff36' },
            px: 1.4, py: 0.4, minHeight: 32,
          }}
        >
          Upload reference
        </Button>
      )}

      {showRerunHint && !adding && (
        <Box sx={{
          px: 1.2, py: 0.9, borderRadius: 1.2,
          bgcolor: 'rgba(229,255,0,0.08)',
          border: `1px solid ${tokens.color.border.accent}`,
        }}>
          <Typography variant="caption" sx={{ color: tokens.color.text.primary, display: 'block', fontWeight: 700 }}>
            Reference saved.
          </Typography>
          <Typography variant="caption" sx={{ color: tokens.color.text.secondary }}>
            Re-run the Finisher (use the Run panel) so the UX gate diffs the new target against the preview.
          </Typography>
        </Box>
      )}

      {adding && (
        <Box sx={{
          p: 1.2, borderRadius: 1.4,
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.inset,
        }}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
            <Typography variant="overline" sx={{ flex: 1, color: tokens.color.text.secondary }}>
              New visual target
            </Typography>
            <IconButton size="small" onClick={() => { setAdding(false); setFormError(null); }}>
              <CloseIcon fontSize="small" />
            </IconButton>
          </Stack>

          <Stack spacing={1}>
            <Button
              component="label"
              size="small"
              variant="outlined"
              startIcon={<ImageOutlined fontSize="small" />}
              sx={{
                alignSelf: 'flex-start',
                color: tokens.color.text.primary,
                borderColor: tokens.color.border.strong,
              }}
            >
              {form.fileName ? 'Replace image' : 'Choose PNG'}
              <input
                type="file"
                accept="image/png,image/jpeg,image/webp"
                hidden
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) void onPickFile(f);
                  e.target.value = '';
                }}
              />
            </Button>

            {form.previewDataUrl && (
              <Box sx={{
                position: 'relative',
                borderRadius: 1.2,
                overflow: 'hidden',
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.surface,
              }}>
                <Box
                  component="img"
                  src={form.previewDataUrl}
                  alt={form.fileName}
                  sx={{ display: 'block', width: '100%', maxHeight: 160, objectFit: 'contain' }}
                />
                <Typography
                  variant="caption"
                  sx={{
                    position: 'absolute', bottom: 4, right: 6,
                    fontFamily: tokens.font.mono, fontSize: 10,
                    color: tokens.color.text.secondary,
                    bgcolor: 'rgba(7,8,7,0.74)',
                    px: 0.6, py: 0.2, borderRadius: 0.6,
                  }}
                >
                  {form.fileName} · {(form.fileBytes / 1024).toFixed(0)} KiB
                </Typography>
              </Box>
            )}

            <TextField
              size="small"
              label="Name"
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              placeholder="Dashboard hero"
              fullWidth
            />
            <TextField
              size="small"
              label="Route hint"
              value={form.routeHint}
              onChange={(e) => setForm((f) => ({ ...f, routeHint: e.target.value }))}
              placeholder="/dashboard"
              fullWidth
              InputProps={{ sx: { fontFamily: tokens.font.mono, fontSize: 12.5 } }}
            />
            <Stack direction="row" spacing={1}>
              <TextField
                size="small"
                label="Viewport W"
                type="number"
                value={form.viewportW}
                onChange={(e) => setForm((f) => ({ ...f, viewportW: Number(e.target.value) || 0 }))}
                fullWidth
              />
              <TextField
                size="small"
                label="Viewport H"
                type="number"
                value={form.viewportH}
                onChange={(e) => setForm((f) => ({ ...f, viewportH: Number(e.target.value) || 0 }))}
                fullWidth
              />
            </Stack>
            <Box>
              <Typography variant="caption" sx={{ color: tokens.color.text.muted }}>
                Tolerance · {(form.tolerance * 100).toFixed(1)}%
              </Typography>
              <Slider
                size="small"
                value={form.tolerance}
                onChange={(_, v) => setForm((f) => ({ ...f, tolerance: Array.isArray(v) ? v[0] : v }))}
                min={0}
                max={0.2}
                step={0.005}
                sx={{ color: tokens.color.accent.lime, mt: 0.4 }}
              />
            </Box>

            {formError && (
              <Typography variant="caption" sx={{ color: tokens.color.accent.danger, fontWeight: 700 }}>
                {formError}
              </Typography>
            )}

            <Stack direction="row" spacing={1} justifyContent="flex-end">
              <Button
                size="small"
                onClick={() => { setAdding(false); setFormError(null); }}
                sx={{ color: tokens.color.text.secondary }}
              >
                Cancel
              </Button>
              <Button
                size="small"
                variant="contained"
                disabled={submitting || !form.base64}
                onClick={onSubmit}
                sx={{
                  bgcolor: tokens.color.accent.lime,
                  color: tokens.color.text.inverse,
                  fontWeight: 800,
                  '&:hover': { bgcolor: '#f0ff36' },
                }}
              >
                {submitting ? 'Uploading…' : 'Upload reference'}
              </Button>
            </Stack>
          </Stack>
        </Box>
      )}

      {loading && (
        <Box sx={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: 0.8,
        }}>
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} variant="rounded" height={110} />
          ))}
        </Box>
      )}

      {!loading && error && (
        <Box sx={{
          px: 1.2, py: 1.2, borderRadius: 1.2,
          bgcolor: 'rgba(255,24,24,0.08)',
        }}>
          <Typography variant="body2" sx={{ color: tokens.color.accent.danger, fontWeight: 700 }}>
            Could not load visual targets.
          </Typography>
          <Typography
            variant="caption"
            onClick={() => void load()}
            sx={{
              color: tokens.color.accent.lime, cursor: 'pointer',
              fontWeight: 700, textTransform: 'uppercase', letterSpacing: '0.06em',
            }}
          >
            Try again
          </Typography>
        </Box>
      )}

      {!loading && !error && items.length === 0 && (
        <Box sx={{
          px: 1.2, py: 1.4, borderRadius: 1.2,
          bgcolor: tokens.color.bg.inset, textAlign: 'center',
        }}>
          <Typography variant="body2" sx={{ fontWeight: 700 }}>
            No references yet
          </Typography>
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.3 }}>
            Upload Figma exports, mockups, or screenshots — the UX gate refuses to ship until the live preview matches.
          </Typography>
        </Box>
      )}

      {!loading && !error && items.length > 0 && (
        <Box sx={{
          display: 'grid',
          gridTemplateColumns: '1fr 1fr',
          gap: 0.8,
        }}>
          {items.map((t) => (
            <VisualTargetCard
              key={t.id}
              target={t}
              busy={busyId === t.id}
              onDelete={() => void onDelete(t.id)}
              onZoom={() => setZoomTarget(t)}
            />
          ))}
        </Box>
      )}

      <Dialog
        open={Boolean(zoomTarget)}
        onClose={() => setZoomTarget(null)}
        maxWidth="lg"
        PaperProps={{
          sx: {
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.subtle}`,
            color: tokens.color.text.primary,
          },
        }}
      >
        {zoomTarget && (
          <Box sx={{ p: 1.4 }}>
            <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Typography variant="overline" color="text.secondary">Visual target</Typography>
                <Typography variant="h6" sx={{ fontWeight: 800 }} noWrap>
                  {zoomTarget.name || `Target ${zoomTarget.id.slice(-6)}`}
                </Typography>
                <Typography variant="caption" sx={{
                  display: 'block',
                  color: tokens.color.text.secondary,
                  fontFamily: tokens.font.mono,
                }}>
                  {(zoomTarget.routeHint || '/')} · {zoomTarget.viewportW}×{zoomTarget.viewportH}
                  {typeof zoomTarget.tolerance === 'number'
                    ? ` · tol ${(zoomTarget.tolerance * 100).toFixed(1)}%`
                    : ''}
                </Typography>
              </Box>
              <IconButton onClick={() => setZoomTarget(null)}>
                <CloseIcon />
              </IconButton>
            </Stack>
            <Box
              component="img"
              src={`data:image/png;base64,${zoomTarget.imagePngBase64}`}
              alt={zoomTarget.name || zoomTarget.id}
              sx={{
                display: 'block',
                maxWidth: '90vw',
                maxHeight: '78vh',
                objectFit: 'contain',
                borderRadius: 1.2,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.inset,
              }}
            />
          </Box>
        )}
      </Dialog>
    </Stack>
  );
}

function VisualTargetCard({
  target, busy, onDelete, onZoom,
}: {
  target: VisualTarget;
  busy: boolean;
  onDelete: () => void;
  onZoom: () => void;
}) {
  const src = `data:image/png;base64,${target.imagePngBase64}`;
  return (
    <Box sx={{
      borderRadius: 1.4,
      border: `1px solid ${tokens.color.border.subtle}`,
      bgcolor: tokens.color.bg.inset,
      overflow: 'hidden',
      display: 'flex',
      flexDirection: 'column',
    }}>
      <Box
        onClick={onZoom}
        sx={{
          cursor: 'zoom-in',
          bgcolor: tokens.color.bg.surface,
          aspectRatio: '16 / 10',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          overflow: 'hidden',
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
        }}
      >
        <Box
          component="img"
          src={src}
          alt={target.name || target.id}
          sx={{
            maxWidth: '100%',
            maxHeight: '100%',
            display: 'block',
            objectFit: 'contain',
          }}
        />
      </Box>
      <Stack spacing={0.2} sx={{ p: 0.9, minWidth: 0 }}>
        <Stack direction="row" alignItems="center" spacing={0.6}>
          <Typography
            variant="caption"
            sx={{ flex: 1, minWidth: 0, fontWeight: 800 }}
            noWrap
            title={target.name || target.id}
          >
            {target.name || `Target ${target.id.slice(-6)}`}
          </Typography>
          <Tooltip title="Delete target">
            <span>
              <IconButton
                size="small"
                disabled={busy}
                onClick={onDelete}
                sx={{
                  p: 0.2,
                  color: tokens.color.text.muted,
                  '&:hover': { color: tokens.color.accent.danger },
                }}
              >
                <DeleteOutline sx={{ fontSize: 16 }} />
              </IconButton>
            </span>
          </Tooltip>
        </Stack>
        <Typography
          variant="caption"
          sx={{
            display: 'block',
            fontFamily: tokens.font.mono,
            fontSize: 10.5,
            color: tokens.color.text.secondary,
            overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
          }}
          title={target.routeHint || '/'}
        >
          {(target.routeHint || '/')} · {target.viewportW}×{target.viewportH}
        </Typography>
        {typeof target.tolerance === 'number' && (
          <Typography variant="caption" sx={{ fontSize: 10.5, color: tokens.color.text.muted }}>
            tol {(target.tolerance * 100).toFixed(1)}%
          </Typography>
        )}
      </Stack>
    </Box>
  );
}

// readImageAsBase64 mirrors the helper in ChatPane — the orchestrator
// expects bare base64 (no `data:` prefix), so we slice the comma off the
// FileReader output before storing.
function readImageAsBase64(file: File): Promise<{ dataUrl: string; base64: string }> {
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onerror = () => reject(r.error ?? new Error('FileReader error'));
    r.onload = () => {
      const dataUrl = String(r.result || '');
      const idx = dataUrl.indexOf(',');
      const base64 = idx >= 0 ? dataUrl.slice(idx + 1) : dataUrl;
      resolve({ dataUrl, base64 });
    };
    r.readAsDataURL(file);
  });
}
