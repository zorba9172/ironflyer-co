import { useMemo } from 'react';
import { Box, Card, Stack, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { Chart, type EChartsOption } from '@ironflyer/ui-web/fx';
import { palette } from '@ironflyer/design-tokens/brand';
import { formatUSD } from '@ironflyer/core';
import { statusLabel, type GateStatus, type StudioProject } from '../studioData';

const SERIES_COLORS = [palette.cobalt, palette.cyan, palette.amber, palette.emerald, palette.rose, '#8a8f99'];

function Stat({ label, value, sub }: { label: string; value: string; sub?: string }) {
  return (
    <Card sx={{ p: 2.5, flex: 1 }}>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.66rem', letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled' })}>{label}</Typography>
      <Typography variant="h4" sx={{ fontSize: '1.9rem', mt: 0.5 }}>{value}</Typography>
      {sub && <Typography sx={{ fontSize: '0.78rem', color: 'text.secondary' }}>{sub}</Typography>}
    </Card>
  );
}

export function UsagePane({ fallback: p }: { fallback: StudioProject }) {
  const theme = useTheme();
  const axis = theme.palette.text.secondary;
  const grid = theme.palette.divider;

  const byKind = useMemo(() => {
    const m = new Map<string, number>();
    for (const e of p.activity) m.set(e.kind, (m.get(e.kind) ?? 0) + 1);
    return [...m.entries()].map(([name, value]) => ({ name, value }));
  }, [p.activity]);

  const gateCounts = useMemo(() => {
    const m = new Map<GateStatus, number>();
    for (const g of p.gates) m.set(g.status, (m.get(g.status) ?? 0) + 1);
    return (['closed', 'running', 'open', 'blocked', 'unstarted'] as GateStatus[]).map((s) => ({ s, n: m.get(s) ?? 0 }));
  }, [p.gates]);

  const eventsOption: EChartsOption = {
    color: SERIES_COLORS,
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, textStyle: { color: axis, fontSize: 11 } },
    series: [{
      type: 'pie', radius: ['52%', '74%'], avoidLabelOverlap: true,
      itemStyle: { borderColor: theme.palette.background.paper, borderWidth: 2 },
      label: { show: false }, data: byKind,
    }],
  };

  const gateOption: EChartsOption = {
    grid: { left: 8, right: 16, top: 16, bottom: 24, containLabel: true },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    xAxis: { type: 'category', data: gateCounts.map((g) => statusLabel[g.s]), axisLabel: { color: axis, fontSize: 10 }, axisLine: { lineStyle: { color: grid } } },
    yAxis: { type: 'value', axisLabel: { color: axis }, splitLine: { lineStyle: { color: grid } } },
    series: [{ type: 'bar', data: gateCounts.map((g, i) => ({ value: g.n, itemStyle: { color: SERIES_COLORS[i % SERIES_COLORS.length], borderRadius: [4, 4, 0, 0] } })), barWidth: '52%' }],
  };

  const walletPct = p.meters.walletBudget ? Math.round((p.meters.walletUsed / p.meters.walletBudget) * 100) : 0;
  const walletOption: EChartsOption = {
    series: [{
      type: 'gauge', startAngle: 210, endAngle: -30, min: 0, max: 100, radius: '92%',
      progress: { show: true, width: 14, itemStyle: { color: palette.cobalt } },
      axisLine: { lineStyle: { width: 14, color: [[1, grid]] } },
      axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false }, pointer: { show: false },
      anchor: { show: false },
      detail: { valueAnimation: true, formatter: `${walletPct}%`, color: theme.palette.text.primary, fontSize: 26, offsetCenter: [0, 0] },
      data: [{ value: walletPct }],
    }],
  };

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Typography variant="h4" sx={{ fontSize: '1.6rem', mb: 0.5 }}>Usage</Typography>
        <Typography sx={{ color: 'text.secondary', mb: 3 }}>Spend, activity, and gate progress at a glance.</Typography>

        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ mb: 2 }}>
          <Stat label="Wallet used" value={formatUSD(p.meters.walletUsed)} sub={`of ${formatUSD(p.meters.walletBudget)} budget`} />
          <Stat label="Margin (30d)" value={`${p.meters.marginPct}%`} sub="revenue − provider cost" />
          <Stat label="Events" value={String(p.activity.length)} sub="orchestration events" />
          <Stat label="Gates closed" value={`${p.gates.filter((g) => !g.blocking).length}/${p.gates.length}`} sub="end-to-end" />
        </Stack>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr 1fr' }, gap: 1.5 }}>
          <Card sx={{ p: 2.5 }}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Wallet budget</Typography>
            <Chart option={walletOption} height={220} />
          </Card>
          <Card sx={{ p: 2.5 }}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Events by source</Typography>
            <Chart option={eventsOption} height={220} />
          </Card>
          <Card sx={{ p: 2.5 }}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Gate status</Typography>
            <Chart option={gateOption} height={220} />
          </Card>
        </Box>
      </Box>
    </Box>
  );
}
