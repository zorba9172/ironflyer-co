import { Box, Card, Stack, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { Chart, type EChartsOption, Reveal } from '@ironflyer/ui-web/fx';
import { palette } from '@ironflyer/design-tokens/brand';
import { formatUSD, formatCompact } from '@ironflyer/core';
import { overview } from '../data';

const SERIES = [palette.cobalt, palette.cyan, palette.amber, palette.emerald, palette.rose];

function Stat({ label, value, sub }: { label: string; value: string; sub: string }) {
  return (
    <Card sx={{ p: 2.5 }}>
      <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.66rem', letterSpacing: '0.08em', textTransform: 'uppercase', color: 'text.disabled' })}>{label}</Typography>
      <Typography variant="h4" sx={{ fontSize: '1.9rem', mt: 0.5 }}>{value}</Typography>
      <Typography sx={{ fontSize: '0.78rem', color: 'text.secondary' }}>{sub}</Typography>
    </Card>
  );
}

export function Overview() {
  const theme = useTheme();
  const axis = theme.palette.text.secondary;
  const grid = theme.palette.divider;

  const stats = [
    { label: 'MRR', value: formatUSD(overview.mrr, { cents: false }), sub: 'recurring revenue' },
    { label: 'Active projects', value: formatCompact(overview.activeProjects), sub: 'running or shipped' },
    { label: 'Provider cost (30d)', value: formatUSD(overview.providerCost30d, { cents: false }), sub: 'metered LLM + sandbox' },
    { label: 'Margin (30d)', value: `${overview.marginPct}%`, sub: 'revenue − provider cost' },
  ];

  const trendOption: EChartsOption = {
    color: [palette.cobalt, palette.rose],
    tooltip: { trigger: 'axis', valueFormatter: (v) => formatUSD(Number(v), { cents: false }) },
    legend: { bottom: 0, textStyle: { color: axis, fontSize: 11 } },
    grid: { left: 8, right: 16, top: 16, bottom: 36, containLabel: true },
    xAxis: { type: 'category', data: overview.months, boundaryGap: false, axisLabel: { color: axis, fontSize: 10 }, axisLine: { lineStyle: { color: grid } } },
    yAxis: { type: 'value', axisLabel: { color: axis, formatter: (v: number) => `$${v / 1000}k` }, splitLine: { lineStyle: { color: grid } } },
    series: [
      { name: 'Revenue', type: 'line', smooth: true, data: overview.revenue, areaStyle: { opacity: 0.12 }, lineStyle: { width: 2.5 }, symbol: 'none' },
      { name: 'Provider cost', type: 'line', smooth: true, data: overview.cost, lineStyle: { width: 2 }, symbol: 'none' },
    ],
  };

  const execOption: EChartsOption = {
    color: SERIES,
    tooltip: { trigger: 'item' },
    legend: { bottom: 0, textStyle: { color: axis, fontSize: 11 } },
    series: [{
      type: 'pie', radius: ['52%', '74%'], avoidLabelOverlap: true,
      itemStyle: { borderColor: theme.palette.background.paper, borderWidth: 2 },
      label: { show: false }, data: overview.executions,
    }],
  };

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <Typography variant="h4" sx={{ fontSize: '1.6rem', mb: 0.5 }}>Overview</Typography>
        <Typography sx={{ color: 'text.secondary', mb: 3 }}>Revenue, spend, and execution health across the platform.</Typography>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 2 }}>
          {stats.map((s, i) => <Reveal key={s.label} delay={i * 60}><Stat {...s} /></Reveal>)}
        </Box>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.6fr 1fr' }, gap: 1.5 }}>
          <Card sx={{ p: 2.5 }}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Revenue vs provider cost</Typography>
            <Chart option={trendOption} height={280} />
          </Card>
          <Card sx={{ p: 2.5 }}>
            <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.68rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1 })}>Executions by status (30d)</Typography>
            <Chart option={execOption} height={280} />
          </Card>
        </Box>
      </Box>
    </Box>
  );
}
