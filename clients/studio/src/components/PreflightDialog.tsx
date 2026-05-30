import { Box, Button, Chip, Dialog, DialogActions, DialogContent, DialogTitle, Divider, Stack, Tooltip, Typography } from '@mui/material';
import { formatUSD } from '@ironflyer/core';
import { VscShield, VscWarning, VscError, VscRocket } from 'react-icons/vsc';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useWallet, useSentinelForecast } from '../hooks/useEconomics';
import { text as fontScale } from '@ironflyer/design-tokens/brand';

type CostEstimate = {
  lowUSD: number; medianUSD: number; highUSD: number; p95USD: number;
  confidence: number; basedOnRuns: number; caveat?: string | null;
};
const ZERO_ESTIMATE: CostEstimate = { lowUSD: 0, medianUSD: 0, highUSD: 0, p95USD: 0, confidence: 0, basedOnRuns: 0, caveat: null };

// H2 — pre-spend plan / cost-preview gate. Before an expensive dispatch the
// operator sees a binding economic verdict (Ironflyer's differentiator over the
// Lovable/Bolt/Cursor "Plan mode" convergence): the prepaid headroom that would
// back the reservation, the live burn trajectory, what the agent intends to do,
// and a ProfitGuard-style verdict — Confirm to spend, Cancel to step back.
//
// This mirrors the confirmAction/confirm-dialog pattern (title · context ·
// Confirm/Cancel) from pages/Editor.tsx, but as a real component so it can carry
// live economics that a SweetAlert string cannot.

type Verdict = 'approve' | 'caution' | 'block';

function verdictMeta(v: Verdict) {
  switch (v) {
    case 'block':
      return { tone: 'error' as const, label: 'ProfitGuard would block', Icon: VscError };
    case 'caution':
      return { tone: 'warning' as const, label: 'Proceed with caution', Icon: VscWarning };
    default:
      return { tone: 'success' as const, label: 'Cleared to spend', Icon: VscShield };
  }
}

export interface PreflightDialogProps {
  open: boolean;
  /** The request the agent is about to act on — shown as "what it intends to do". */
  prompt: string;
  /** Chat mode selected for this dispatch. */
  mode?: 'ask' | 'plan' | 'execute' | 'autopilot';
  /** Live project the dispatch reserves against (drives the forecast). */
  projectId: string | null;
  /** Whether the studio is connected to the orchestrator (sample mode otherwise). */
  isLive: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export function PreflightDialog({ open, prompt, mode = 'plan', projectId, isLive, onConfirm, onCancel }: PreflightDialogProps) {
  const { wallet } = useWallet();
  const { forecast } = useSentinelForecast(open ? projectId : null);

  // Real per-dispatch cost forecast (estimateExecutionCost → CostEstimate),
  // fetched only while the dialog is open and live. An empty input yields the
  // tenant/global baseline; medianUSD is the expected spend, p95USD the
  // worst-case the operator should be ready for.
  const { data: est, isLive: estLive } = useGraphQLQuery<CostEstimate, { estimateExecutionCost: CostEstimate }>({
    key: ['estimateExecutionCost', mode],
    operationName: 'EstimateExecutionCost',
    query: operations.ESTIMATE_EXECUTION_COST,
    variables: { input: {} },
    fallbackData: ZERO_ESTIMATE,
    map: (raw) => raw.estimateExecutionCost,
    enabled: open && isLive,
  });
  const hasEstimate = estLive && est.medianUSD > 0;

  const available = wallet.availableUSD;
  const headroom = forecast.remainingHeadroomUSD;
  const burning = forecast.burnRatePerHourUSD > 0;
  const projected = forecast.extrapolatedTotalUSD;

  // Binding economic verdict: a dry wallet blocks (law 1, 402); an expected
  // cost that exceeds the available balance blocks (can't afford the run); a
  // critical/over-headroom trajectory or estimate near the balance cautions;
  // otherwise cleared.
  const noBudget = available <= 0;
  const cantAfford = !noBudget && hasEstimate && est.medianUSD > available;
  const tight =
    !noBudget && !cantAfford &&
    (forecast.level === 'warn' || forecast.level === 'critical' || headroom < 1 || (hasEstimate && est.p95USD > available));
  const verdict: Verdict = noBudget || cantAfford ? 'block' : tight ? 'caution' : 'approve';
  const { tone, label, Icon } = verdictMeta(verdict);

  const intent = prompt.trim().length > 220 ? `${prompt.trim().slice(0, 220)}…` : prompt.trim();

  return (
    <Dialog
      open={open}
      onClose={onCancel}
      maxWidth="xs"
      fullWidth
      slotProps={{ paper: { sx: { border: 1, borderColor: 'divider', backgroundImage: 'none' } } }}
    >
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', gap: 1, pb: 1 }}>
        <Box sx={(t) => ({ display: 'grid', placeItems: 'center', width: 30, height: 30, borderRadius: 2, color: 'common.white', backgroundImage: t.brand.gradient.signature })}>
          <VscRocket size={15} />
        </Box>
        <Box sx={{ minWidth: 0 }}>
          <Typography sx={{ fontWeight: 700, fontSize: fontScale.s100 }}>Review before dispatch</Typography>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s66, color: 'text.disabled' })}>
            pre-spend gate · prepaid wallet reservation
          </Typography>
        </Box>
      </DialogTitle>

      <DialogContent dividers sx={{ p: 2.5 }}>
        {/* What the agent intends to do */}
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: fontScale.s62, letterSpacing: '0.1em', textTransform: 'uppercase', color: 'text.disabled', mb: 0.75 })}>
          The agent will work on
        </Typography>
        <Box sx={{ p: 1.25, borderRadius: 2, bgcolor: 'action.hover', mb: 2 }}>
          <Typography sx={{ fontSize: fontScale.s84, color: 'text.primary', whiteSpace: 'pre-wrap', lineHeight: 1.5 }}>
            {intent || 'the open work'}
          </Typography>
        </Box>

        {/* Economic preview — bound to live wallet + sentinel forecast */}
        <Stack spacing={1.25}>
          <CostLine label="Dispatch mode" value={mode} muted={mode !== 'autopilot'} />
          <CostLine
            label="Estimated cost"
            value={!isLive ? 'Sample mode' : hasEstimate ? `≈ ${formatUSD(est.medianUSD)} · p95 ${formatUSD(est.p95USD)}` : 'Estimate warming up'}
            muted={!hasEstimate}
            tone={cantAfford ? 'error' : undefined}
            tip={hasEstimate ? `Expected (median) spend, with the p95 worst case. Based on ${est.basedOnRuns} prior run(s). Final reservation locks when the run starts.` : 'Final reservation is locked when the run starts.'}
          />
          <CostLine label="Wallet available" value={isLive ? formatUSD(available) : '—'} />
          <CostLine
            label="Headroom this run"
            value={isLive && (burning || headroom !== 0) ? formatUSD(headroom) : '—'}
            tone={isLive && headroom < 0 ? 'error' : undefined}
          />
          {isLive && burning && (
            <CostLine label="Burn / projected" value={`${formatUSD(forecast.burnRatePerHourUSD)}/h · ${formatUSD(projected)}`} muted />
          )}
        </Stack>

        <Divider sx={{ my: 2 }} />

        {/* Binding ProfitGuard-style verdict */}
        <Stack direction="row" alignItems="center" spacing={1}>
          <Box sx={(t) => ({ color: t.palette[tone].main, display: 'flex' })}>
            <Icon size={16} />
          </Box>
          <Chip
            size="small"
            label={isLive ? label : 'Verdict needs a live connection'}
            sx={(t) => ({
              height: 22,
              fontSize: fontScale.s70,
              fontWeight: 600,
              color: isLive ? t.palette[tone].main : t.palette.text.secondary,
              bgcolor: `${t.palette[tone].main}1f`,
            })}
          />
        </Stack>
        <Typography sx={{ fontSize: fontScale.s78, color: 'text.secondary', mt: 1, lineHeight: 1.5 }}>
          {!isLive
            ? 'Offline preview — connect the orchestrator to see the live wallet, burn trajectory, and ProfitGuard verdict before you spend.'
            : verdict === 'block'
              ? cantAfford
                ? `The expected cost (${formatUSD(est.medianUSD)}) exceeds your available balance (${formatUSD(available)}). ProfitGuard will refuse this run at admission — top up before confirming.`
                : 'Your wallet is out of credit. ProfitGuard will reject this paid dispatch (402). Top up before confirming.'
              : verdict === 'caution'
                ? 'Budget headroom is tight on the current trajectory. You can proceed, but the run may exhaust the wallet mid-flight.'
                : 'There is enough prepaid headroom to back this dispatch. Confirm to reserve and run.'}
        </Typography>
      </DialogContent>

      <DialogActions sx={{ px: 2.5, py: 1.5 }}>
        <Button color="inherit" onClick={onCancel}>Cancel</Button>
        <Button
          variant="contained"
          color={verdict === 'block' ? 'error' : 'primary'}
          onClick={onConfirm}
        >
          {verdict === 'block' ? 'Dispatch anyway' : 'Confirm & dispatch'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function CostLine({ label, value, muted, tone, tip }: { label: string; value: string; muted?: boolean; tone?: 'error'; tip?: string }) {
  const row = (
    <Stack direction="row" alignItems="baseline" justifyContent="space-between" spacing={2}>
      <Typography sx={{ fontSize: fontScale.s82, color: 'text.secondary' }}>{label}</Typography>
      <Typography
        sx={(t) => ({
          fontFamily: t.brand.font.mono,
          fontSize: fontScale.s82,
          fontWeight: 600,
          color: tone === 'error' ? t.palette.error.main : muted ? 'text.disabled' : 'text.primary',
          textAlign: 'right',
          minWidth: 0,
        })}
      >
        {value}
      </Typography>
    </Stack>
  );
  return tip ? <Tooltip title={tip} arrow placement="top">{row}</Tooltip> : row;
}
