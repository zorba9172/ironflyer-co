'use client';

// PreviewPane — iframe with toolbar: refresh, open-in-new-tab, viewport size.
// URL resolution lives in lib/api/runtime-preview so backend changes don't
// leak into UI components.

import { useEffect, useMemo, useState } from 'react';
import {
  Box, Button, IconButton, MenuItem, Select, Skeleton, Stack, Tooltip, Typography,
} from '@mui/material';
import {
  DesktopWindows, Laptop, OpenInNew, Refresh, Smartphone, TabletMac,
} from '@mui/icons-material';
import { Workspace } from '../../lib/runtime';
import { PortMapping, getWorkspacePorts, resolvePreviewURL } from '../../lib/api/runtime-preview';
import { tokens } from '../../lib/theme';

type Device = 'mobile' | 'tablet' | 'desktop';

interface Props {
  workspace: Workspace | null;
  // when the run completes, the parent bumps this to force a soft refresh
  refreshKey?: number;
}

const DEVICE_SIZES: Record<Device, { w: number; h: number; label: string }> = {
  mobile:  { w: 390,  h: 780,  label: 'Mobile · 390' },
  tablet:  { w: 820,  h: 1180, label: 'Tablet · 820' },
  desktop: { w: 1280, h: 800,  label: 'Desktop · 1280' },
};

export function PreviewPane({ workspace, refreshKey = 0 }: Props) {
  const [device, setDevice] = useState<Device>('desktop');
  const [ports, setPorts] = useState<PortMapping[]>([]);
  const [token, setToken] = useState<string | undefined>(undefined);
  const [selectedPort, setSelectedPort] = useState<number | undefined>(undefined);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [bump, setBump] = useState(0);

  useEffect(() => {
    if (!workspace) {
      setPorts([]); setToken(undefined); setError(null); setLoading(false);
      return;
    }
    let alive = true;
    setLoading(true);
    setError(null);
    getWorkspacePorts(workspace.id)
      .then((res) => {
        if (!alive) return;
        setPorts(res.ports);
        setToken(res.previewToken);
        if (!selectedPort && res.ports[0]) setSelectedPort(res.ports[0].port);
      })
      .catch((e) => {
        if (!alive) return;
        setError(String(e?.message ?? e));
      })
      .finally(() => alive && setLoading(false));
    return () => { alive = false; };
  }, [workspace?.id, refreshKey]);

  const url = useMemo(
    () => resolvePreviewURL(workspace, ports, token, selectedPort),
    [workspace, ports, token, selectedPort],
  );

  const size = DEVICE_SIZES[device];

  if (!workspace) {
    return (
      <EmptyShell
        title="עדיין אין סביבת ריצה"
        body="הריצו את ה־Finisher או פתחו את Terminal כדי להפעיל סביבה — התצוגה תיטען כאן ברגע שתעלה השרת."
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
                  :{p.port} {p.ready ? '· ready' : '· starting…'}
                </MenuItem>
              ))}
            </Select>
          ) : (
            <Typography variant="caption" sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>
              {workspace.previewUrl ? `legacy preview` : 'awaiting a forwarded port'}
            </Typography>
          )}
        </Box>
        <Tooltip title="רענון תצוגה">
          <span>
            <IconButton size="small" disabled={!url} onClick={() => setBump((b) => b + 1)}>
              <Refresh fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
        <Tooltip title="פתיחה בכרטיסייה חדשה">
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
            title="התצוגה תופיע כאן"
            body="ברגע שהסוכן מפעיל שרת תצוגה, הוא יוצג בלייב — כולל hot reload."
            inset
          />
        ) : (
          <Box sx={{
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
              style={{ width: '100%', height: '100%', border: 0, display: 'block', background: '#fff' }}
            />
          </Box>
        )}
      </Box>
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

function ErrorState({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <Stack spacing={1.2} alignItems="center" sx={{ textAlign: 'center', px: 3, maxWidth: 360 }}>
      <Typography variant="subtitle1" sx={{ fontWeight: 800, color: tokens.color.accent.danger }}>
        לא הצלחנו לטעון את התצוגה
      </Typography>
      <Typography variant="caption" sx={{ color: tokens.color.text.muted, maxWidth: 320 }} title={message}>
        {message.slice(0, 220)}
      </Typography>
      <Button variant="contained" size="small" onClick={onRetry}>נסו שוב</Button>
    </Stack>
  );
}
