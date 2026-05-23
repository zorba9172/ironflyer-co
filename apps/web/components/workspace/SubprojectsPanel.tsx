'use client';

// SubprojectsPanel — surfaces the per-project subprojects API inside the
// workspace sidebar. Subprojects model a monorepo layout (one app per
// directory). The orchestrator uses each `path` to claim generated files
// onto the right service at execution time.
//
// Visual rhythm matches WorkspaceSidebar: alabaster surface, lime accent
// for the add CTA, mono font for paths. The panel is self-contained — it
// owns its loading / error / form state so the parent shell can keep its
// dependency graph small.

import { useCallback, useEffect, useState } from 'react';
import {
  Box, Button, Chip, IconButton, MenuItem, Skeleton, Stack, TextField,
  Tooltip, Typography,
} from '@mui/material';
import {
  Add as AddIcon, DeleteOutline, Close as CloseIcon,
} from '@mui/icons-material';

import { api, Subproject } from '../../lib/api';
import { tokens } from '../../lib/theme';

interface Props {
  projectId: string;
}

const ROLES = ['frontend', 'backend', 'worker', 'mobile', 'ml', 'other'] as const;

interface FormState {
  name: string;
  path: string;
  role: string;
  frontend: string;
  backend: string;
  storage: string;
  auth: string;
}

const EMPTY_FORM: FormState = {
  name: '',
  path: '',
  role: 'frontend',
  frontend: '',
  backend: '',
  storage: '',
  auth: '',
};

export function SubprojectsPanel({ projectId }: Props) {
  const [items, setItems] = useState<Subproject[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [adding, setAdding] = useState(false);
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await api.listSubprojects(projectId));
    } catch (e) {
      setError(String((e as Error)?.message ?? e));
    } finally {
      setLoading(false);
    }
  }, [projectId]);

  useEffect(() => { void load(); }, [load]);

  const onSubmit = useCallback(async () => {
    if (!form.name.trim() || !form.path.trim()) {
      setFormError('Name and path are required.');
      return;
    }
    setSubmitting(true);
    setFormError(null);
    try {
      const stack: NonNullable<Parameters<typeof api.addSubproject>[1]['stack']> = {};
      if (form.frontend.trim()) stack.frontend = form.frontend.trim();
      if (form.backend.trim())  stack.backend  = form.backend.trim();
      if (form.storage.trim())  stack.storage  = form.storage.trim();
      if (form.auth.trim())     stack.auth     = form.auth.trim();
      await api.addSubproject(projectId, {
        name: form.name.trim(),
        path: form.path.trim(),
        role: form.role,
        stack: Object.keys(stack).length ? stack : undefined,
      });
      setForm(EMPTY_FORM);
      setAdding(false);
      await load();
    } catch (e) {
      setFormError(String((e as Error)?.message ?? e));
    } finally {
      setSubmitting(false);
    }
  }, [form, load, projectId]);

  const onDelete = useCallback(async (subId: string) => {
    setBusyId(subId);
    try {
      await api.deleteSubproject(projectId, subId);
      await load();
    } catch (e) {
      setError(String((e as Error)?.message ?? e));
    } finally {
      setBusyId(null);
    }
  }, [load, projectId]);

  const onPathClick = useCallback((path: string) => {
    // Best-effort: emit a custom event for any listener (the file tree may
    // grow to subscribe), and fall back to scrolling the page so the user
    // gets immediate visual feedback that the click was received.
    if (typeof window !== 'undefined') {
      try {
        window.dispatchEvent(new CustomEvent('ironflyer:focus-path', { detail: { path } }));
      } catch {
        // older browsers without CustomEvent constructor — ignore.
      }
      window.scrollTo({ top: 0, behavior: 'smooth' });
    }
  }, []);

  return (
    <Stack spacing={1.2}>
      {!adding && (
        <Button
          size="small"
          startIcon={<AddIcon />}
          onClick={() => { setAdding(true); setFormError(null); }}
          sx={{
            alignSelf: 'flex-start',
            bgcolor: tokens.color.accent.lime,
            color: tokens.color.text.inverse,
            fontWeight: 800,
            '&:hover': { bgcolor: '#f0ff36' },
            px: 1.4, py: 0.4, minHeight: 32,
          }}
        >
          Add subproject
        </Button>
      )}

      {adding && (
        <Box sx={{
          p: 1.2, borderRadius: 1.4,
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: tokens.color.bg.inset,
        }}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
            <Typography variant="overline" sx={{ flex: 1, color: tokens.color.text.secondary }}>
              New subproject
            </Typography>
            <IconButton size="small" onClick={() => { setAdding(false); setFormError(null); }}>
              <CloseIcon fontSize="small" />
            </IconButton>
          </Stack>
          <Stack spacing={1}>
            <TextField
              size="small"
              label="Name"
              value={form.name}
              onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              placeholder="apps/web"
              fullWidth
            />
            <TextField
              size="small"
              label="Path"
              value={form.path}
              onChange={(e) => setForm((f) => ({ ...f, path: e.target.value }))}
              placeholder="apps/web"
              fullWidth
              InputProps={{ sx: { fontFamily: tokens.font.mono, fontSize: 12.5 } }}
            />
            <TextField
              size="small"
              select
              label="Role"
              value={form.role}
              onChange={(e) => setForm((f) => ({ ...f, role: e.target.value }))}
              fullWidth
            >
              {ROLES.map((r) => (
                <MenuItem key={r} value={r}>{r}</MenuItem>
              ))}
            </TextField>
            <Typography variant="caption" sx={{ color: tokens.color.text.muted, mt: 0.4 }}>
              Stack (optional)
            </Typography>
            <Stack direction="row" spacing={1}>
              <TextField
                size="small"
                label="Frontend"
                value={form.frontend}
                onChange={(e) => setForm((f) => ({ ...f, frontend: e.target.value }))}
                placeholder="next.js"
                fullWidth
              />
              <TextField
                size="small"
                label="Backend"
                value={form.backend}
                onChange={(e) => setForm((f) => ({ ...f, backend: e.target.value }))}
                placeholder="go"
                fullWidth
              />
            </Stack>
            <Stack direction="row" spacing={1}>
              <TextField
                size="small"
                label="Storage"
                value={form.storage}
                onChange={(e) => setForm((f) => ({ ...f, storage: e.target.value }))}
                placeholder="postgres"
                fullWidth
              />
              <TextField
                size="small"
                label="Auth"
                value={form.auth}
                onChange={(e) => setForm((f) => ({ ...f, auth: e.target.value }))}
                placeholder="jwt"
                fullWidth
              />
            </Stack>
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
                disabled={submitting}
                onClick={onSubmit}
                sx={{
                  bgcolor: tokens.color.accent.lime,
                  color: tokens.color.text.inverse,
                  fontWeight: 800,
                  '&:hover': { bgcolor: '#f0ff36' },
                }}
              >
                {submitting ? 'Saving…' : 'Add subproject'}
              </Button>
            </Stack>
          </Stack>
        </Box>
      )}

      {loading && (
        <Stack spacing={0.6}>
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} variant="rounded" height={64} />
          ))}
        </Stack>
      )}

      {!loading && error && (
        <Box sx={{
          px: 1.2, py: 1.2, borderRadius: 1.2,
          bgcolor: 'rgba(255,24,24,0.08)',
        }}>
          <Typography variant="body2" sx={{ color: tokens.color.accent.danger, fontWeight: 700 }}>
            Could not load subprojects.
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
            No subprojects yet
          </Typography>
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.3 }}>
            Use subprojects to model a monorepo — one app per directory.
          </Typography>
        </Box>
      )}

      {!loading && !error && items.length > 0 && (
        <Stack spacing={0.8}>
          {items.map((s) => (
            <SubprojectCard
              key={s.id}
              sub={s}
              busy={busyId === s.id}
              onDelete={() => void onDelete(s.id)}
              onPathClick={() => onPathClick(s.path)}
            />
          ))}
        </Stack>
      )}
    </Stack>
  );
}

function SubprojectCard({
  sub, busy, onDelete, onPathClick,
}: {
  sub: Subproject;
  busy: boolean;
  onDelete: () => void;
  onPathClick: () => void;
}) {
  const stackSummary = sub.stack
    ? [sub.stack.frontend, sub.stack.backend, sub.stack.storage, sub.stack.auth]
        .filter(Boolean).join(' · ')
    : '';
  return (
    <Box sx={{
      p: 1.1, borderRadius: 1.4,
      border: `1px solid ${tokens.color.border.subtle}`,
      bgcolor: tokens.color.bg.inset,
    }}>
      <Stack direction="row" alignItems="flex-start" spacing={1}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Stack direction="row" spacing={0.8} alignItems="center" sx={{ mb: 0.4 }}>
            <Typography variant="body2" sx={{ fontWeight: 800 }} noWrap title={sub.name}>
              {sub.name}
            </Typography>
            {sub.role && (
              <Chip
                label={sub.role}
                size="small"
                sx={{
                  height: 18, fontSize: 10, fontWeight: 800,
                  textTransform: 'uppercase', letterSpacing: '0.04em',
                  bgcolor: 'rgba(229,255,0,0.16)',
                  color: tokens.color.accent.lime,
                  border: `1px solid ${tokens.color.border.accent}`,
                  '& .MuiChip-label': { px: 0.8 },
                }}
              />
            )}
          </Stack>
          <Tooltip title="Reveal in tree">
            <Typography
              variant="caption"
              onClick={onPathClick}
              sx={{
                display: 'block',
                fontFamily: tokens.font.mono,
                fontSize: 11.5,
                color: tokens.color.text.secondary,
                cursor: 'pointer',
                '&:hover': { color: tokens.color.accent.lime },
                overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
              }}
              title={sub.path}
            >
              {sub.path}
            </Typography>
          </Tooltip>
          {stackSummary && (
            <Typography
              variant="caption"
              color="text.secondary"
              sx={{ display: 'block', mt: 0.3 }}
              noWrap
              title={stackSummary}
            >
              {stackSummary}
            </Typography>
          )}
        </Box>
        <Tooltip title="Delete subproject">
          <span>
            <IconButton
              size="small"
              disabled={busy}
              onClick={onDelete}
              sx={{ color: tokens.color.text.muted, '&:hover': { color: tokens.color.accent.danger } }}
            >
              <DeleteOutline fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      </Stack>
    </Box>
  );
}
