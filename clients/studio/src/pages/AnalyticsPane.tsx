import { useMemo, useState } from 'react';
import { Box, Card, Stack, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { Chart, type EChartsOption } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';

interface MetricPoint { ts: string; visitors: number; pageViews: number; sessions: number }
interface PageStat { path: string; views: number; avgSeconds: number }
interface ReferrerStat { source: string; visitors: number }
interface EventStat { name: string; count: number; conversionPct: number }
interface Analytics {
  rangeDays: number; visitors: number; pageViews: number; sessions: number;
  bounceRatePct: number; avgSessionSeconds: number; visitorsDeltaPct: number;
  series: MetricPoint[]; topPages: PageStat[]; topReferrers: ReferrerStat[]; events: EventStat[];
}

function sample(days: number): Analytics {
  const series: MetricPoint[] = Array.from({ length: days }, (_, i) => {
    const v = 200 + Math.round(120 * Math.sin(i / 4) + i * 4);
    return { ts: new Date(Date.now() - (days - 1 - i) * 864e5).toISOString(), visitors: v, pageViews: v * 3, sessions: Math.round(v * 1.3) };
  });
  return {
    rangeDays: days, visitors: series.reduce((a, p) => a + p.visitors, 0), pageViews: series.reduce((a, p) => a + p.pageViews, 0),
    sessions: series.reduce((a, p) => a + p.sessions, 0), bounceRatePct: 42.3, avgSessionSeconds: 96.4, visitorsDeltaPct: 12.5,
    series, topPages: [{ path: '/', views: 4210, avgSeconds: 38 }, { path: '/pricing', views: 1980, avgSeconds: 64 }, { path: '/signup', views: 1240, avgSeconds: 22 }],
    topReferrers: [{ source: 'Direct', visitors: 2100 }, { source: 'Google', visitors: 1640 }, { source: 'Product Hunt', visitors: 720 }],
    events: [{ name: 'signup', count: 412, conversionPct: 8.4 }, { name: 'purchase', count: 96, conversionPct: 2.0 }],
  };
}

export function AnalyticsPane() {
  const t = useTheme();
  const liveProjectId = useOperateProjectId();
  const [days, setDays] = useState(30);

  const { data, isLive } = useGraphQLQuery<Analytics, { appAnalytics: Analytics }>({
    key: ['app-analytics', liveProjectId ?? 'none', days],
    operationName: 'AppAnalytics', query: operations.APP_ANALYTICS,
    variables: { projectID: liveProjectId, days }, fallbackData: sample(days), enabled: !!liveProjectId,
    map: (r) => r.appAnalytics ?? sample(days),
  });

  const trend = useMemo<EChartsOption>(() => ({
    grid: { left: 44, right: 16, top: 24, bottom: 28 },
    tooltip: { trigger: 'axis' },
    legend: { top: 0, right: 0, textStyle: { color: t.palette.text.secondary, fontSize: 11 } },
    xAxis: { type: 'category', boundaryGap: false, data: data.series.map((p) => new Date(p.ts).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })), axisLabel: { color: t.palette.text.disabled, fontSize: 10 }, axisLine: { lineStyle: { color: t.palette.divider } } },
    yAxis: { type: 'value', axisLabel: { color: t.palette.text.disabled, fontSize: 10 }, splitLine: { lineStyle: { color: t.palette.divider } } },
    series: [
      { name: 'Visitors', type: 'line', smooth: true, showSymbol: false, areaStyle: { opacity: 0.12 }, lineStyle: { width: 2, color: t.brand.accent.primary }, itemStyle: { color: t.brand.accent.primary }, data: data.series.map((p) => p.visitors) },
      { name: 'Page views', type: 'line', smooth: true, showSymbol: false, lineStyle: { width: 2, color: t.brand.accent.secondary }, itemStyle: { color: t.brand.accent.secondary }, data: data.series.map((p) => p.pageViews) },
    ],
  }), [data, t]);

  const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));
  const metrics = [
    { label: 'Visitors', value: fmt(data.visitors), sub: `${data.visitorsDeltaPct >= 0 ? '+' : ''}${data.visitorsDeltaPct}% vs prior`, delta: data.visitorsDeltaPct },
    { label: 'Page views', value: fmt(data.pageViews), sub: `${(data.pageViews / Math.max(data.sessions, 1)).toFixed(1)} / session` },
    { label: 'Bounce rate', value: `${data.bounceRatePct}%`, sub: 'single-page sessions' },
    { label: 'Avg session', value: `${Math.floor(data.avgSessionSeconds / 60)}m ${Math.round(data.avgSessionSeconds % 60)}s`, sub: 'time on app' },
  ];

  const ranked = (rows: { label: string; value: string; n: number }[], max: number) => (
    <Stack spacing={1}>
      {rows.map((r) => (
        <Box key={r.label}>
          <Stack direction="row" justifyContent="space-between" sx={{ mb: 0.25 }}>
            <Typography sx={{ fontSize: '0.82rem' }} noWrap>{r.label}</Typography>
            <Typography sx={{ fontSize: '0.8rem', color: 'text.secondary' }}>{r.value}</Typography>
          </Stack>
          <Box sx={{ height: 5, borderRadius: 9, bgcolor: 'action.hover', overflow: 'hidden' }}>
            <Box sx={(th) => ({ height: '100%', width: `${Math.round((r.n / Math.max(max, 1)) * 100)}%`, backgroundImage: th.brand.gradient.signature })} />
          </Box>
        </Box>
      ))}
    </Stack>
  );

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader
          title="Analytics"
          isLive={isLive}
          actions={
            <ToggleButtonGroup size="small" exclusive value={days} onChange={(_, v) => v && setDays(v)}>
              {[7, 30, 90].map((dd) => <ToggleButton key={dd} value={dd} sx={{ textTransform: 'none', px: 1.5, py: 0.25 }}>{dd}d</ToggleButton>)}
            </ToggleButtonGroup>
          }
        />

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 1.5 }}>
          {metrics.map((m) => (
            <Card key={m.label} sx={{ p: 2.5 }}>
              <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled' })}>{m.label}</Typography>
              <Typography variant="h4" sx={{ fontSize: '1.7rem', mt: 0.5 }}>{m.value}</Typography>
              <Typography sx={{ fontSize: '0.74rem', color: m.delta != null ? (m.delta >= 0 ? 'success.main' : 'error.main') : 'text.secondary' }}>{m.sub}</Typography>
            </Card>
          ))}
        </Box>

        <Card sx={{ p: 2, mb: 1.5 }}>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>Traffic</Typography>
          <Chart option={trend} height={260} />
        </Card>

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.5 }}>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Top pages</Typography>
            {ranked(data.topPages.map((p) => ({ label: p.path, value: fmt(p.views), n: p.views })), data.topPages[0]?.views ?? 1)}
          </Card>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Referrers</Typography>
            {ranked(data.topReferrers.map((p) => ({ label: p.source, value: fmt(p.visitors), n: p.visitors })), data.topReferrers[0]?.visitors ?? 1)}
          </Card>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Conversion events</Typography>
            {ranked(data.events.map((e) => ({ label: e.name, value: `${e.count} · ${e.conversionPct}%`, n: e.count })), data.events[0]?.count ?? 1)}
          </Card>
        </Box>
      </Box>
    </Box>
  );
}
