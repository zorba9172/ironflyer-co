import { useState, type ReactNode } from 'react';
import { Box, Button, Chip, IconButton, MenuItem, Popover, Select, Stack, Tooltip, Typography } from '@mui/material';
import { useTheme, type Theme } from '@mui/material/styles';
import { VscPlay, VscDebugStop, VscInfo, VscWand, VscRobot, VscChevronRight } from 'react-icons/vsc';
import { statusColor } from '../statusColor';
import { TechIcon } from '../../lib/techIcons';
import { statusLabel, type Gate } from '../../studioData';

// Theme-sourced color set for the dense node bodies (React Flow nodes render
// outside MUI's sx pipeline, so values are read from the theme here).
export interface MapColors {
  mono: string; muted: string; primary: string; secondary: string;
  track: string; success: string; warn: string; error: string; accent: string; paper: string;
}
export function nodePalette(t: Theme): MapColors {
  return {
    mono: t.brand.font.mono,
    muted: t.palette.text.disabled,
    primary: t.palette.text.primary,
    secondary: t.palette.text.secondary,
    track: t.palette.action.hover,
    success: t.palette.success.main,
    warn: t.palette.warning.main,
    error: t.palette.error.main,
    accent: t.palette.primary.main,
    paper: t.palette.background.paper,
  };
}

const Row = ({ children }: { children: ReactNode }) => (
  <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>{children}</div>
);

// --- Gate node ----------------------------------------------------------
export interface GateNodeData {
  kind: 'gate';
  gate: Gate;
  ownerName?: string;
  agentOptions: { id: string; name: string }[];
  online: boolean;
  onSelectGate: (id: string) => void;
  onDispatch: (scope: string) => void;
}

export function GateNodeLabel({ d }: { d: GateNodeData }) {
  const t = useTheme();
  const c = nodePalette(t);
  const { gate: g, ownerName, agentOptions, onSelectGate, onDispatch } = d;
  const color = statusColor(t, g.status);
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [running, setRunning] = useState(g.status === 'running');
  const [agentId, setAgentId] = useState<string>('');

  const stop = (e: { stopPropagation: () => void }) => e.stopPropagation();
  const toggleRun = (e: React.MouseEvent) => {
    stop(e);
    setRunning((r) => !r);
    if (!running) onDispatch(`the ${g.name} gate`);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 7, textAlign: 'left', width: '100%' }}>
      <Row>
        <Box component="span" sx={{ color: 'text.secondary', display: 'inline-flex' }}><TechIcon name={g.id} size={15} title={g.name} /></Box>
        <span style={{ fontFamily: c.mono, fontSize: 10, color: c.muted }}>{g.no}</span>
        <span style={{ flex: 1 }} />
        <span style={{ fontFamily: c.mono, fontSize: 9, fontWeight: 700, letterSpacing: '0.06em', color }}>{statusLabel[g.status].toUpperCase()}</span>
      </Row>

      <div style={{ fontSize: 13.5, fontWeight: 600, color: c.primary, lineHeight: 1.15 }}>{g.name}</div>

      {ownerName && (
        <Row>
          <VscRobot size={11} color={c.muted} />
          <span style={{ fontFamily: c.mono, fontSize: 9.5, color: c.muted }}>{ownerName}</span>
        </Row>
      )}

      <div style={{ height: 4, borderRadius: 99, background: c.track, overflow: 'hidden' }}>
        <div style={{ height: '100%', width: `${Math.round(g.level * 100)}%`, background: color, transition: 'width .4s ease' }} />
      </div>

      <div style={{ fontSize: 10.5, lineHeight: 1.3, color: g.blocking ? c.secondary : c.success, display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', overflow: 'hidden', minHeight: 14 }}>
        {g.blocking ? `● ${g.blocking}` : '● Closed end-to-end'}
      </div>

      {/* controls — nodrag/nopan so the canvas doesn't steal the interaction */}
      <Stack className="nodrag nopan" direction="row" alignItems="center" spacing={0.5} sx={{ mt: 0.25 }}>
        <Tooltip title={running ? 'Stop agent' : 'Run agent'} arrow>
          <IconButton size="small" onClick={toggleRun} sx={{ color: running ? 'error.main' : 'success.main', p: 0.4 }}>
            {running ? <VscDebugStop size={15} /> : <VscPlay size={15} />}
          </IconButton>
        </Tooltip>

        <Select
          size="small" variant="standard" disableUnderline displayEmpty
          value={agentId}
          onClick={stop}
          onChange={(e) => { setAgentId(e.target.value); }}
          renderValue={(v) => {
            const a = agentOptions.find((x) => x.id === v);
            return <span style={{ fontFamily: c.mono, fontSize: 9.5, color: a ? c.primary : c.muted }}>{a ? a.name : 'Assign agent'}</span>;
          }}
          sx={{ flex: 1, minWidth: 0, '& .MuiSelect-select': { py: 0.2, pl: 0.5 } }}
          MenuProps={{ slotProps: { paper: { sx: { maxHeight: 280 } } } }}
        >
          {agentOptions.map((a) => <MenuItem key={a.id} value={a.id} sx={{ fontSize: '0.8rem' }}>{a.name}</MenuItem>)}
        </Select>

        <Tooltip title="Findings & patches" arrow>
          <IconButton size="small" onClick={(e) => { stop(e); setAnchor(e.currentTarget); }} sx={{ color: 'text.secondary', p: 0.4 }}>
            <VscInfo size={14} />
          </IconButton>
        </Tooltip>

        {g.blocking && (
          <Tooltip title="Dispatch a fix" arrow>
            <IconButton size="small" onClick={(e) => { stop(e); onDispatch(`the ${g.name} gate`); }} sx={{ color: 'primary.main', p: 0.4 }}>
              <VscWand size={14} />
            </IconButton>
          </Tooltip>
        )}
      </Stack>

      <Popover
        open={!!anchor} anchorEl={anchor} onClose={() => setAnchor(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        slotProps={{ paper: { sx: { width: 280, p: 2, border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}
      >
        <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
          <Box component="span" sx={{ color: 'text.secondary', display: 'inline-flex' }}><TechIcon name={g.id} size={16} title={g.name} /></Box>
          <Typography sx={{ fontWeight: 600, fontSize: '0.95rem', flex: 1 }}>{g.name}</Typography>
          <Chip size="small" label={statusLabel[g.status]} sx={{ height: 18, fontSize: '0.6rem', bgcolor: `${color}22`, color }} />
        </Stack>
        {g.findings.length === 0 ? (
          <Typography sx={{ fontSize: '0.82rem', color: 'success.main' }}>● No open findings.</Typography>
        ) : (
          <Stack spacing={0.75} sx={{ mb: 1.5 }}>
            {g.findings.map((f) => (
              <Stack key={f.id} direction="row" spacing={0.85} alignItems="flex-start">
                <Box component="span" sx={{ mt: '3px', color: f.severity === 'danger' ? 'error.main' : f.severity === 'warning' ? 'warning.main' : 'text.disabled' }}>●</Box>
                <Typography sx={{ fontSize: '0.8rem', color: 'text.secondary' }}>{f.text}</Typography>
              </Stack>
            ))}
          </Stack>
        )}
        <Stack direction="row" spacing={1}>
          <Button fullWidth size="small" variant="outlined" color="inherit" onClick={() => { setAnchor(null); onSelectGate(g.id); }}>Open inspector</Button>
          {g.blocking && <Button fullWidth size="small" variant="contained" onClick={() => { setAnchor(null); onDispatch(`the ${g.name} gate`); }}>Fix</Button>}
        </Stack>
      </Popover>
    </div>
  );
}

// --- Facet node (Security / Performance / Logs / Economics) -------------
export interface FacetNodeData {
  kind: 'facet';
  iconKey: string;
  title: string;
  metric: string;
  sub?: string;
  accent: string;
  onOpen: () => void;
  details: { label: string; value: string; color?: string }[];
}

export function FacetNodeLabel({ d }: { d: FacetNodeData }) {
  const c = nodePalette(useTheme());
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, textAlign: 'left', width: '100%' }}>
      <Row>
        <Box component="span" sx={{ color: d.accent, display: 'inline-flex' }}><TechIcon name={d.iconKey} size={15} title={d.title} /></Box>
        <span style={{ fontFamily: c.mono, fontSize: 9, fontWeight: 700, letterSpacing: '0.1em', color: d.accent }}>{d.title.toUpperCase()}</span>
        <span style={{ flex: 1 }} />
        <VscChevronRight size={13} color={c.muted} />
      </Row>
      <div style={{ fontSize: 18, fontWeight: 700, color: c.primary, lineHeight: 1.05 }}>{d.metric}</div>
      {d.sub && <div style={{ fontSize: 10, color: c.secondary, lineHeight: 1.25 }}>{d.sub}</div>}
      <Stack className="nodrag nopan" direction="row" alignItems="center" spacing={0.5} sx={{ mt: 0.25 }}>
        <Button size="small" variant="text" onClick={(e) => { e.stopPropagation(); d.onOpen(); }} sx={{ minWidth: 0, px: 0.75, fontSize: '0.68rem', color: 'primary.main' }}>Open</Button>
        <span style={{ flex: 1 }} />
        <Tooltip title="Breakdown" arrow>
          <IconButton size="small" onClick={(e) => { e.stopPropagation(); setAnchor(e.currentTarget); }} sx={{ color: 'text.secondary', p: 0.4 }}>
            <VscInfo size={14} />
          </IconButton>
        </Tooltip>
      </Stack>
      <Popover
        open={!!anchor} anchorEl={anchor} onClose={() => setAnchor(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        slotProps={{ paper: { sx: { width: 240, p: 2, border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}
      >
        <Typography sx={{ fontWeight: 600, fontSize: '0.9rem', mb: 1 }}>{d.title}</Typography>
        <Stack spacing={0.85}>
          {d.details.map((row) => (
            <Stack key={row.label} direction="row" justifyContent="space-between" alignItems="baseline">
              <Typography sx={{ fontSize: '0.78rem', color: 'text.secondary' }}>{row.label}</Typography>
              <Typography sx={{ fontFamily: 'var(--if-font-mono)', fontSize: '0.78rem', fontWeight: 600, color: row.color ?? 'text.primary' }}>{row.value}</Typography>
            </Stack>
          ))}
        </Stack>
        <Button fullWidth size="small" variant="outlined" color="inherit" sx={{ mt: 1.5 }} onClick={() => { setAnchor(null); d.onOpen(); }}>Open {d.title}</Button>
      </Popover>
    </div>
  );
}

// --- Vision + Ship + Agent (simple, non-interactive bodies) -------------
export function VisionBody({ text, c }: { text: string; c: MapColors }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, textAlign: 'left', width: '100%' }}>
      <Row>
        <VscWand size={13} color={c.accent} />
        <span style={{ fontFamily: c.mono, fontSize: 9.5, fontWeight: 700, letterSpacing: '0.1em', color: c.accent }}>VISION</span>
      </Row>
      <div style={{ fontSize: 12.5, color: c.primary, lineHeight: 1.3, display: '-webkit-box', WebkitLineClamp: 5, WebkitBoxOrient: 'vertical', overflow: 'hidden' }}>{text}</div>
    </div>
  );
}

export function ShipBody({ url, color, shippable, open, c }: { url: string; color: string; shippable: boolean; open: number; c: MapColors }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6, textAlign: 'left', width: '100%' }}>
      <span style={{ fontFamily: c.mono, fontSize: 9.5, fontWeight: 700, letterSpacing: '0.1em', color }}>SHIP</span>
      <div style={{ fontSize: 13.5, fontWeight: 600, color: c.primary }}>{url}</div>
      <div style={{ fontSize: 10.5, color: shippable ? c.success : c.secondary, lineHeight: 1.3 }}>
        {shippable ? '● All gates closed — shippable' : `● ${open} gate${open === 1 ? '' : 's'} block shipping`}
      </div>
    </div>
  );
}
