import { useMemo, useState } from 'react';
import { Box, Button, Chip, Divider, LinearProgress, Popover, Stack, Tooltip, Typography } from '@mui/material';
import { useNavigate } from 'react-router-dom';
import { motion } from '@ironflyer/ui-web/fx';
import { formatUSD, formatRelativeTime } from '@ironflyer/core';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useWallet, useSentinelForecast, buildActionCostPreview, buildGateSpendLabels, type GateSpendLabel } from '../hooks/useEconomics';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useProjectExecutions } from '../hooks/useLatestExecution';
import { useStudio } from '../store';
import { agentForGate } from '../studioData';
import { text } from '@ironflyer/design-tokens/brand';

// Only the most recent entries are rendered — capped, never unbounded — so the
// popover stays a glanceable receipt, not a log viewer (the full ledger lives in
// the dashboard Usage tab).
const LEDGER_LIMIT = 6;

interface LedgerEntry {
  id: string;
  executionID?: string | null;
  entryType: string;
  direction: string;
  amountUSD: number;
  billable: boolean;
  createdAt: string;
}

// Human labels for the canonical ledger entry-type strings. Anything not mapped
// falls back to a de-snaked version so a new backend type still reads cleanly.
const ENTRY_LABEL: Record<string, string> = {
  provider_cost: 'Model call',
  premium_reasoning: 'Premium reasoning',
  sandbox_cost: 'Sandbox',
  storage_cost: 'Storage',
  deployment_cost: 'Deploy',
  reservation: 'Reservation hold',
  release: 'Hold released',
  refund: 'Refund',
  topup: 'Top-up',
  top_up: 'Top-up',
  revenue: 'Revenue',
};

function labelFor(entryType: string): string {
  return ENTRY_LABEL[entryType] ?? entryType.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
}

// A debit (money leaving the wallet) is what users feel as "cost"; credits and
// released holds are relief. We tint accordingly so the receipt reads calm.
function isDebit(e: LedgerEntry): boolean {
  return e.direction === 'debit' && e.amountUSD > 0;
}

// Always-visible cost HUD: prepaid wallet balance + live burn rate, glanceable
// while you work. The category's #1 complaint is the surprise bill — no
// competitor surfaces live spend at all times. The pill is the always-on
// reassurance; clicking it opens the receipt that answers "what did that
// click cost" — wallet, burn, headroom, and the last few real ledger lines.
export function CostHUD() {
  const navigate = useNavigate();
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const fallbackProject = useStudio((s) => s.current);
  const firstProjectId = useLiveProjectId();
  const projectId = storeProjectId ?? firstProjectId;
  const { wallet, isLive } = useWallet();
  const { forecast } = useSentinelForecast(projectId);
  const { executions, economics } = useProjectExecutions(projectId);

  // Anchor lives in state (not a ref) so the Popover reads it outside render —
  // refs must not be accessed during render (react-hooks/refs).
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const open = Boolean(anchorEl);

  // Recent ledger lines — only fetched while the popover is open, and capped.
  const { data: ledger, isLive: ledgerLive } = useGraphQLQuery<LedgerEntry[], { ledger: LedgerEntry[] }>({
    key: ['costhud-ledger'],
    operationName: 'Ledger',
    query: operations.LEDGER,
    variables: { filter: { limit: LEDGER_LIMIT } },
    fallbackData: [],
    enabled: open,
    refetchInterval: open ? 8000 : undefined,
    map: (r) => r.ledger ?? [],
  });

  const recent = useMemo(() => ledger.slice(0, LEDGER_LIMIT), [ledger]);
  const preview = useMemo(() => buildActionCostPreview({
    wallet,
    forecast,
    recentExecutionSpendUSD: executions.map((e) => e.spentUSD || e.budgetUSD),
    fallbackEstimateUSD: fallbackProject.profitGuard.reservedUSD || 2.4,
  }), [wallet, forecast, executions, fallbackProject.profitGuard.reservedUSD]);

  const burning = forecast.burnRatePerHourUSD > 0;
  const overHeadroom = preview.verdict === 'blocked' || forecast.remainingHeadroomUSD < 0;
  const tight = preview.verdict === 'tight';
  const tone: 'error' | 'warning' | 'success' = overHeadroom ? 'error' : tight ? 'warning' : 'success';
  const gateSpend = useMemo(() => buildGateSpendLabels(
    fallbackProject.gates.map((g) => ({ ...g, agentName: agentForGate(g.id)?.name })),
    economics.spentUSD,
    fallbackProject.meters.walletUsed,
  ).sort((a, b) => b.amountUSD - a.amountUSD).slice(0, 4), [fallbackProject.gates, fallbackProject.meters.walletUsed, economics.spentUSD]);

  // Hold ratio for the wallet bar: how much of the balance is reserved right now.
  const holdPct = wallet.balanceUSD > 0 ? Math.min(100, Math.max(0, (wallet.holdUSD / wallet.balanceUSD) * 100)) : 0;
  const topUp = () => {
    setAnchorEl(null);
    navigate('/plans');
  };

  const tip = overHeadroom
    ? 'Payment required protection: top up before the next paid execution (402).'
    : `Next action ${formatUSD(preview.estimateUSD)} · ${formatUSD(preview.headroomAfterUSD)} headroom after`;

  return (
    <>
      <Tooltip title={isLive ? tip : 'Connect to see live wallet + burn'} arrow disableInteractive>
        <Stack
          role="button"
          tabIndex={0}
          aria-label="Open cost receipt"
          onClick={(e) => setAnchorEl(e.currentTarget)}
          onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setAnchorEl(e.currentTarget); } }}
          direction="row"
          alignItems="center"
          spacing={1}
          sx={(t) => ({
            px: 1.25, py: 0.5, borderRadius: 99,
            border: 1, borderColor: open ? `${t.palette[tone].main}66` : 'divider', bgcolor: 'action.hover',
            cursor: 'pointer', userSelect: 'none',
            color: t.palette[tone].main,
            transition: `border-color ${t.brand.motion.fast}`,
            '&:hover': { borderColor: `${t.palette[tone].main}66` },
          })}
        >
          <Box sx={{ position: 'relative', width: 8, height: 8, flexShrink: 0 }}>
            <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: 'currentColor' }} />
            {burning && !overHeadroom && (
              <Box
                component={motion.span}
                animate={{ scale: [1, 2.4], opacity: [0.55, 0] }}
                transition={{ duration: 1.4, repeat: Infinity, ease: 'easeOut' }}
                sx={{ position: 'absolute', inset: 0, borderRadius: 99, bgcolor: 'currentColor' }}
              />
            )}
          </Box>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s74, fontWeight: 600, color: 'text.primary' })}>
            {formatUSD(wallet.availableUSD)}
          </Typography>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s70, color: t.palette[tone].main })}>
            next {formatUSD(preview.estimateUSD)}
          </Typography>
          {burning && (
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s70, color: t.palette[tone].main })}>
              {formatUSD(forecast.burnRatePerHourUSD)}/h
            </Typography>
          )}
        </Stack>
      </Tooltip>

      <Popover
        open={open}
        anchorEl={anchorEl}
        onClose={() => setAnchorEl(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        slotProps={{ paper: { sx: { mt: 1, width: 380, maxWidth: '92vw', borderRadius: 2, border: 1, borderColor: 'divider', backgroundImage: 'none', overflow: 'hidden' } } }}
      >
        {/* Header — title + live/offline affordance */}
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ px: 2, pt: 1.75, pb: 1.25 }}>
          <Stack>
            <Typography variant="overline" sx={{ color: 'text.secondary', lineHeight: 1.2 }}>Cost cockpit</Typography>
            <Typography sx={{ fontSize: text.s78, color: 'text.disabled' }}>Pre-spend estimate, headroom, and receipt.</Typography>
          </Stack>
          <Chip
            size="small"
            label={isLive ? 'live' : 'offline'}
            sx={(t) => ({ height: 20, fontSize: text.s62, fontFamily: t.brand.font.mono, bgcolor: isLive ? `${t.palette.success.main}22` : 'action.hover', color: isLive ? 'success.main' : 'text.disabled' })}
          />
        </Stack>

        <Divider />

        {/* Pre-action economics — the thing to check before a paid dispatch. */}
        <Stack sx={{ px: 2, py: 1.75 }} spacing={1.25}>
          <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={2}>
            <Stack sx={{ minWidth: 0 }}>
              <Typography sx={{ fontSize: text.s74, color: 'text.secondary' }}>{preview.label}</Typography>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s66, color: 'text.disabled' })} noWrap>
                {preview.detail}
              </Typography>
            </Stack>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s130, fontWeight: 700, color: t.palette[tone].main, flexShrink: 0 })}>
              {formatUSD(preview.estimateUSD)}
            </Typography>
          </Stack>

          <Box>
            <LinearProgress variant="determinate" value={preview.coveragePct} color={tone} sx={{ height: 5, borderRadius: 99, bgcolor: 'action.hover' }} />
            <Stack direction="row" justifyContent="space-between" sx={{ mt: 0.75 }}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s66, color: 'text.disabled' })}>
                before {formatUSD(preview.headroomBeforeUSD)}
              </Typography>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s66, color: preview.headroomAfterUSD < 0 ? t.palette.error.main : 'text.disabled' })}>
                after {formatUSD(preview.headroomAfterUSD)}
              </Typography>
            </Stack>
          </Box>

          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            spacing={1.25}
            sx={(t) => ({
              px: 1.25, py: 1, borderRadius: 1.5,
              bgcolor: preview.verdict === 'blocked' ? `${t.palette.error.main}14` : preview.verdict === 'tight' ? `${t.palette.warning.main}14` : 'action.hover',
            })}
          >
            <Typography sx={{ fontSize: text.s74, color: preview.verdict === 'blocked' ? 'error.main' : preview.verdict === 'tight' ? 'warning.main' : 'text.secondary', lineHeight: 1.35 }}>
              {preview.verdict === 'blocked'
                ? '402 protection: paid work pauses until the wallet can cover the reservation.'
                : preview.actionText}
            </Typography>
            {preview.verdict === 'blocked' && (
              <Button size="small" variant="contained" color="error" onClick={topUp} sx={{ flexShrink: 0 }}>
                Top up
              </Button>
            )}
          </Stack>
        </Stack>

        <Divider />

        {/* Wallet — available is the number that matters; hold + balance for context */}
        <Stack sx={{ px: 2, py: 1.75 }} spacing={1.25}>
          <Stack direction="row" alignItems="baseline" justifyContent="space-between">
            <Typography sx={{ fontSize: text.s74, color: 'text.secondary' }}>Available to spend</Typography>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s95, fontWeight: 700, color: t.palette[tone].main })}>
              {formatUSD(wallet.availableUSD)}
            </Typography>
          </Stack>

          <Box>
            <LinearProgress
              variant="determinate"
              value={holdPct}
              color={tone}
              sx={{ height: 4, borderRadius: 99, bgcolor: 'action.hover' }}
            />
            <Stack direction="row" justifyContent="space-between" sx={{ mt: 0.75 }}>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s66, color: 'text.disabled' })}>
                {formatUSD(wallet.holdUSD)} held
              </Typography>
              <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s66, color: 'text.disabled' })}>
                {formatUSD(wallet.balanceUSD)} balance
              </Typography>
            </Stack>
          </Box>
        </Stack>

        <Divider />

        {/* Trajectory — burn / headroom / projected, from the live forecast */}
        <Stack direction="row" sx={{ px: 2, py: 1.5 }} spacing={2}>
          <Figure label="Burn / hour" value={formatUSD(forecast.burnRatePerHourUSD)} tone={burning ? tone : undefined} pulse={burning && !overHeadroom} />
          <Figure label="After next" value={formatUSD(preview.headroomAfterUSD)} tone={overHeadroom ? 'error' : tight ? 'warning' : undefined} />
          <Figure label="Projected" value={formatUSD(forecast.extrapolatedTotalUSD)} />
        </Stack>

        <Divider />

        {/* Spend allocation — live execution spend when available, fallback gate shares otherwise. */}
        <Stack sx={{ px: 2, pt: 1.25, pb: 0.5 }} direction="row" alignItems="center" justifyContent="space-between">
          <Typography variant="overline" sx={{ color: 'text.secondary' }}>Spend by agent</Typography>
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, color: 'text.disabled' })}>
            {gateSpend[0]?.source === 'live' ? 'live' : gateSpend[0]?.source === 'fallback' ? 'sample' : 'allocated'}
          </Typography>
        </Stack>
        <Stack sx={{ px: 1, pb: 1.25 }} spacing={0.25}>
          {gateSpend.map((g) => <SpendRow key={g.gateId} item={g} />)}
        </Stack>

        <Divider />

        {/* Recent ledger lines — deliberately quiet; the dashboard owns the full usage view. */}
        <Stack sx={{ px: 2, pt: 1.25, pb: 0.5 }} direction="row" alignItems="center" justifyContent="space-between">
          <Typography variant="overline" sx={{ color: 'text.disabled' }}>Recent receipt</Typography>
          {ledgerLive && recent.length > 0 && (
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, color: 'text.disabled' })}>
              last {recent.length}
            </Typography>
          )}
        </Stack>

        <Box sx={{ maxHeight: 148, overflowY: 'auto', px: 1, pb: 1.25 }}>
          {recent.length === 0 ? (
            <Typography sx={{ px: 1, py: 1.5, fontSize: text.s74, color: 'text.disabled' }}>
              {isLive ? 'No charges yet — nothing has cost you a cent.' : 'Connect to see itemized charges.'}
            </Typography>
          ) : (
            <Stack>
              {recent.map((e) => (
                <LedgerRow key={e.id} entry={e} />
              ))}
            </Stack>
          )}
        </Box>

        <Divider />

        {/* Calm reassurance — this is the category's relief, not an alarm */}
        <Typography sx={{ px: 2, py: 1.25, fontSize: text.s66, color: 'text.disabled', lineHeight: 1.4 }}>
          {overHeadroom
            ? 'Wallet is exhausted. Paid executions pause at 402 until you top up; nothing runs that you cannot cover.'
            : 'Every paid step reserves from your wallet first, then debits exactly what it used. No silent charges.'}
        </Typography>
      </Popover>
    </>
  );
}

function Figure(props: { label: string; value: string; tone?: 'error' | 'warning' | 'success'; pulse?: boolean }) {
  const { label, value, tone, pulse } = props;
  return (
    <Stack spacing={0.25} sx={{ flex: 1, minWidth: 0 }}>
      <Stack direction="row" alignItems="center" spacing={0.5}>
        <Typography sx={{ fontSize: text.s64, color: 'text.disabled' }} noWrap>{label}</Typography>
        {pulse && (
          <Box
            component={motion.span}
            animate={{ opacity: [0.4, 1, 0.4] }}
            transition={{ duration: 1.6, repeat: Infinity, ease: 'easeInOut' }}
            sx={(t) => ({ width: 5, height: 5, borderRadius: 99, bgcolor: tone ? t.palette[tone].main : 'text.disabled', flexShrink: 0 })}
          />
        )}
      </Stack>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s80, fontWeight: 600, color: tone ? t.palette[tone].main : 'text.primary' })} noWrap>
        {value}
      </Typography>
    </Stack>
  );
}

function SpendRow({ item }: { item: GateSpendLabel }) {
  return (
    <Stack direction="row" alignItems="center" spacing={1} sx={{ px: 1, py: 0.65, borderRadius: 1.5 }}>
      <Box sx={{ width: 6, height: 6, borderRadius: 99, flexShrink: 0, bgcolor: 'primary.main', opacity: item.amountUSD > 0 ? 0.9 : 0.35 }} />
      <Stack sx={{ flex: 1, minWidth: 0 }}>
        <Typography sx={{ fontSize: text.s74, color: 'text.primary' }} noWrap>{item.agentName}</Typography>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, color: 'text.disabled' })} noWrap>
          {item.gateName} gate · {item.sharePct}% share
        </Typography>
      </Stack>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s72, color: item.amountUSD > 0 ? 'text.secondary' : 'text.disabled', flexShrink: 0 })}>
        {formatUSD(item.amountUSD)}
      </Typography>
    </Stack>
  );
}

function LedgerRow({ entry }: { entry: LedgerEntry }) {
  const debit = isDebit(entry);
  return (
    <Stack
      direction="row"
      alignItems="center"
      spacing={1}
      sx={{ px: 1, py: 0.65, borderRadius: 1.5, opacity: 0.82, '&:hover': { bgcolor: 'action.hover', opacity: 1 } }}
    >
      <Box sx={(t) => ({ width: 6, height: 6, borderRadius: 99, flexShrink: 0, bgcolor: debit ? t.palette.text.disabled : t.palette.success.main })} />
      <Stack sx={{ flex: 1, minWidth: 0 }}>
        <Typography sx={{ fontSize: text.s74, color: 'text.primary' }} noWrap>
          {labelFor(entry.entryType)}
        </Typography>
        {/* Provider is intentionally NOT shown — the orchestrator speaks for
            every vendor; the user sees the charge category + time, never which
            provider ran it. */}
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: text.s62, color: 'text.disabled' })} noWrap>
          {formatRelativeTime(entry.createdAt)}
        </Typography>
      </Stack>
      <Typography
        sx={(t) => ({
          fontFamily: t.brand.font.mono,
          fontSize: text.s74,
          fontWeight: 600,
          color: debit ? t.palette.text.primary : t.palette.success.main,
          flexShrink: 0,
        })}
      >
        {debit ? '' : '+'}{formatUSD(entry.amountUSD)}
      </Typography>
    </Stack>
  );
}
