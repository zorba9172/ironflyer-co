import { useMemo, useState } from 'react';
import { Box, Chip, Stack, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { useGraphQLQuery, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';
import { StudioChart, horizontalBarOption, lineTrendOption, type EChartsOption } from '../components/charts';
import { GlassPanel, StatCard, SectionHeader } from '../components/studio';
import { text } from '@ironflyer/design-tokens/brand';

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
    series, topPages: [{ path: '/', views: 4210, avgSeconds: 38 }, { path: '/pricing', views: 1980, avgSeconds: 64 }, { path: '/signup', views: 1240, avgSeconds: 22 }, { path: '/docs', views: 820, avgSeconds: 118 }, { path: '/blog', views: 540, avgSeconds: 93 }],
    topReferrers: [{ source: 'Direct', visitors: 2100 }, { source: 'Google', visitors: 1640 }, { source: 'Product Hunt', visitors: 720 }, { source: 'Twitter/X', visitors: 380 }],
    events: [{ name: 'signup', count: 412, conversionPct: 8.4 }, { name: 'purchase', count: 96, conversionPct: 2.0 }, { name: 'invite_sent', count: 203, conversionPct: 4.2 }],
  };
}

const fmt = (n: number) => (n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n));

function RankedRow({ label, value, n, max, tone }: { label: string; value: string; n: number; max: number; tone: string }) {
  return (
    <Box>
      <Stack direction="row" justifyContent="space-between" alignItems="center" sx={{ mb: 0.4 }}>
        <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s80 })} noWrap>{label}</Typography>
        <Typography sx={{ fontSize: text.s78, color: 'text.secondary' }}>{value}</Typography>
      </Stack>
      <Box sx={{ height: 5, borderRadius: 9, bgcolor: 'action.hover', overflow: 'hidden' }}>
        <Box sx={{ height: '100%', width: `${Math.round((n / Math.max(max, 1)) * 100)}%`, bgcolor: tone, borderRadius: 9, transition: 'width 400ms cubic-bezier(0.22,0.61,0.36,1)' }} />
      </Box>
    </Box>
  );
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

  const trend = useMemo<EChartsOption>(() => lineTrendOption(t, {
    categories: data.series.map((p) => new Date(p.ts).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })),
    series: [
      { name: 'Visitors', data: data.series.map((p) => p.visitors), area: true },
      { name: 'Page views', data: data.series.map((p) => p.pageViews) },
    ],
  }), [data, t]);

  const pageBarData = useMemo<EChartsOption>(() => horizontalBarOption(t, {
    labels: data.topPages.slice(0, 6).map((p) => p.path),
    values: data.topPages.slice(0, 6).map((p) => p.views),
  }), [data.topPages, t]);

  const metrics = [
    { label: 'Visitors', value: fmt(data.visitors), delta: data.visitorsDeltaPct, accent: t.studio.neon.blue },
    { label: 'Page views', value: fmt(data.pageViews), hint: `${(data.pageViews / Math.max(data.sessions, 1)).toFixed(1)} / session`, accent: t.studio.neon.violet },
    { label: 'Bounce rate', value: `${data.bounceRatePct}%`, hint: 'single-page sessions', accent: t.palette.warning.main },
    { label: 'Avg session', value: `${Math.floor(data.avgSessionSeconds / 60)}m ${Math.round(data.avgSessionSeconds % 60)}s`, hint: 'time on app', accent: t.studio.neon.success },
  ];

  const pageMax = data.topPages[0]?.views ?? 1;
  const refMax = data.topReferrers[0]?.visitors ?? 1;
  const evtMax = data.events[0]?.count ?? 1;

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1120, mx: 'auto' }}>
        <PaneHeader
          title="Analytics"
          isLive={isLive}
          actions={
            <ToggleButtonGroup size="small" exclusive value={days} onChange={(_, v) => v && setDays(v)}>
              {[7, 30, 90].map((dd) => (
                <ToggleButton key={dd} value={dd} sx={{ textTransform: 'none', px: 1.5, py: 0.25 }}>{dd}d</ToggleButton>
              ))}
            </ToggleButtonGroup>
          }
        />

        {/* KPI strip */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 2.5 }}>
          {metrics.map((m) => (
            <StatCard
              key={m.label}
              label={m.label}
              value={m.value}
              delta={m.delta}
              hint={m.hint}
              accent={m.accent}
            />
          ))}
        </Box>

        {/* Hero visual: flat traffic volume bars bound to topPages values */}
        <GlassPanel pad={2.5} sx={{ mb: 2.5 }}>
          <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1.5 }}>
            <Box>
              <Typography
                sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', letterSpacing: '0.12em', color: 'text.disabled' })}
              >
                Top pages — traffic volume
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 0.25 }}>
                Each bar represents page-view count for the selected period.
              </Typography>
            </Box>
            <Chip
              size="small"
              label={`${data.topPages.length} pages`}
              sx={(th) => ({ height: 22, fontSize: text.s68, bgcolor: `${th.studio.neon.violet}1a`, color: 'primary.main' })}
            />
          </Stack>
          <StudioChart option={pageBarData} height={280} />
        </GlassPanel>

        {/* Trend chart */}
        <GlassPanel pad={2.5} sx={{ mb: 2.5 }}>
          <Typography
            sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', letterSpacing: '0.12em', color: 'text.disabled', mb: 1 })}
          >
            Traffic trend
          </Typography>
          <StudioChart option={trend} height={250} />
        </GlassPanel>

        {/* Three breakdown panels */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.5 }}>
          <GlassPanel pad={2.5}>
            <SectionHeader eyebrow="Pages" title="Top pages" />
            <Stack spacing={1.5}>
              {data.topPages.map((p) => (
                <RankedRow key={p.path} label={p.path} value={fmt(p.views)} n={p.views} max={pageMax} tone={t.studio.neon.violet} />
              ))}
            </Stack>
          </GlassPanel>

          <GlassPanel pad={2.5}>
            <SectionHeader eyebrow="Sources" title="Referrers" />
            <Stack spacing={1.5}>
              {data.topReferrers.map((r) => (
                <RankedRow key={r.source} label={r.source} value={fmt(r.visitors)} n={r.visitors} max={refMax} tone={t.studio.neon.blue} />
              ))}
            </Stack>
          </GlassPanel>

          <GlassPanel pad={2.5}>
            <SectionHeader eyebrow="Events" title="Conversions" />
            <Stack spacing={1.5}>
              {data.events.map((e) => (
                <RankedRow key={e.name} label={e.name} value={`${e.count} · ${e.conversionPct}%`} n={e.count} max={evtMax} tone={t.studio.neon.success} />
              ))}
            </Stack>
          </GlassPanel>
        </Box>
      </Box>
    </Box>
  );
}
