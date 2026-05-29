import { useEffect, useMemo, useState } from 'react';
import { Box, Button, Card, FormControlLabel, Stack, Switch, TextField, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { useQueryClient } from '@tanstack/react-query';
import { Chart, type EChartsOption, toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';

interface SeoSettings { projectID: string; title: string; description: string; keywords: string[]; ogImageURL: string; twitterHandle: string; canonicalURL: string; robots: string; sitemapEnabled: boolean; updatedAt: string }
interface SeoCheck { key: string; label: string; passed: boolean; detail: string }
interface SeoAudit { score: number; checks: SeoCheck[] }

const SAMPLE_SETTINGS: SeoSettings = { projectID: '', title: 'TaskFlow — projects that ship', description: '', keywords: [], ogImageURL: '', twitterHandle: '', canonicalURL: '', robots: 'index,follow', sitemapEnabled: true, updatedAt: '' };
const SAMPLE_AUDIT: SeoAudit = { score: 43, checks: [
  { key: 'title', label: 'Title tag', passed: true, detail: 'good length' },
  { key: 'description', label: 'Meta description', passed: false, detail: 'missing description' },
  { key: 'keywords', label: 'Keywords', passed: false, detail: 'no keywords set' },
  { key: 'og_image', label: 'Open Graph image', passed: false, detail: 'no og:image — links unfurl blank' },
  { key: 'canonical', label: 'Canonical URL', passed: false, detail: 'missing canonical URL' },
  { key: 'sitemap', label: 'Sitemap', passed: true, detail: 'sitemap.xml served' },
  { key: 'robots', label: 'Robots policy', passed: true, detail: 'index,follow' },
] };

export function MarketingPane() {
  const t = useTheme();
  const request = useRequest();
  const qc = useQueryClient();
  const liveProjectId = useOperateProjectId();
  const [busy, setBusy] = useState(false);
  const [draft, setDraft] = useState<SeoSettings | null>(null);

  const { data: settings, isLive } = useGraphQLQuery<SeoSettings, { appSeoSettings: SeoSettings }>({
    key: ['app-seo', liveProjectId ?? 'none'], operationName: 'AppSeoSettings', query: operations.APP_SEO_SETTINGS,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE_SETTINGS, enabled: !!liveProjectId, map: (r) => r.appSeoSettings ?? SAMPLE_SETTINGS,
  });
  const { data: audit } = useGraphQLQuery<SeoAudit, { appSeoAudit: SeoAudit }>({
    key: ['app-seo-audit', liveProjectId ?? 'none'], operationName: 'AppSeoAudit', query: operations.APP_SEO_AUDIT,
    variables: { projectID: liveProjectId }, fallbackData: SAMPLE_AUDIT, enabled: !!liveProjectId, map: (r) => r.appSeoAudit ?? SAMPLE_AUDIT,
  });

  useEffect(() => { setDraft(settings); }, [settings.updatedAt, settings.projectID]);
  const d = draft ?? settings;
  const set = (patch: Partial<SeoSettings>) => setDraft({ ...d, ...patch });

  const save = async () => {
    if (!request || !liveProjectId) { toast('Connect the orchestrator to save SEO.', 'error'); return; }
    setBusy(true);
    try {
      await request('UpdateAppSeoSettings', operations.UPDATE_APP_SEO_SETTINGS, { projectID: liveProjectId, input: {
        title: d.title, description: d.description, keywords: d.keywords, ogImageURL: d.ogImageURL,
        twitterHandle: d.twitterHandle, canonicalURL: d.canonicalURL, robots: d.robots, sitemapEnabled: d.sitemapEnabled,
      } });
      void qc.invalidateQueries({ queryKey: ['app-seo', liveProjectId] });
      void qc.invalidateQueries({ queryKey: ['app-seo-audit', liveProjectId] });
      toast('SEO saved.', 'success');
    } catch (e) { toast(e instanceof Error ? e.message : 'Save failed.', 'error'); }
    finally { setBusy(false); }
  };

  const scoreColor = audit.score >= 80 ? t.palette.success.main : audit.score >= 50 ? t.palette.warning.main : t.palette.error.main;
  const gauge = useMemo<EChartsOption>(() => ({
    series: [{
      type: 'gauge', startAngle: 210, endAngle: -30, min: 0, max: 100, radius: '100%',
      progress: { show: true, width: 14, itemStyle: { color: scoreColor } },
      axisLine: { lineStyle: { width: 14, color: [[1, t.palette.action.hover]] } },
      axisTick: { show: false }, splitLine: { show: false }, axisLabel: { show: false }, pointer: { show: false },
      anchor: { show: false },
      detail: { valueAnimation: true, fontSize: 30, offsetCenter: [0, 0], color: scoreColor, formatter: '{value}' },
      data: [{ value: audit.score }],
    }],
  }), [audit.score, scoreColor, t]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader title="Marketing" isLive={isLive} subtitle="SEO & social metadata for the deployed app" />

        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '300px 1fr' }, gap: 1.5, mb: 3, alignItems: 'stretch' }}>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 0.5 })}>SEO score</Typography>
            <Chart option={gauge} height={170} />
            <Typography sx={{ textAlign: 'center', fontSize: '0.78rem', color: 'text.secondary', mt: -1 }}>{audit.checks.filter((c) => c.passed).length}/{audit.checks.length} checks passing</Typography>
          </Card>
          <Card sx={{ p: 2 }}>
            <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Audit</Typography>
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 1 }}>
              {audit.checks.map((c) => (
                <Stack key={c.key} direction="row" alignItems="center" spacing={1}>
                  <Box sx={{ color: c.passed ? 'success.main' : 'warning.main', fontSize: '0.9rem' }}>{c.passed ? '✓' : '○'}</Box>
                  <Box sx={{ minWidth: 0 }}>
                    <Typography sx={{ fontSize: '0.8rem' }}>{c.label}</Typography>
                    <Typography sx={{ fontSize: '0.7rem', color: 'text.disabled' }} noWrap>{c.detail}</Typography>
                  </Box>
                </Stack>
              ))}
            </Box>
          </Card>
        </Box>

        <Card sx={{ p: 2.5 }}>
          <Typography sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: '0.66rem', textTransform: 'uppercase', color: 'text.disabled', mb: 1.5 })}>Metadata</Typography>
          <Stack spacing={1.5}>
            <TextField size="small" label="Page title" value={d.title} onChange={(e) => set({ title: e.target.value })} helperText={`${d.title.length}/60`} />
            <TextField size="small" label="Meta description" value={d.description} onChange={(e) => set({ description: e.target.value })} multiline minRows={2} helperText={`${d.description.length}/160`} />
            <TextField size="small" label="Keywords (comma-separated)" value={d.keywords.join(', ')} onChange={(e) => set({ keywords: e.target.value.split(',').map((k) => k.trim()).filter(Boolean) })} />
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 1.5 }}>
              <TextField size="small" label="OG image URL" value={d.ogImageURL} onChange={(e) => set({ ogImageURL: e.target.value })} />
              <TextField size="small" label="Twitter handle" value={d.twitterHandle} onChange={(e) => set({ twitterHandle: e.target.value })} />
              <TextField size="small" label="Canonical URL" value={d.canonicalURL} onChange={(e) => set({ canonicalURL: e.target.value })} />
              <TextField size="small" label="Robots" value={d.robots} onChange={(e) => set({ robots: e.target.value })} />
            </Box>
            <Stack direction="row" alignItems="center" justifyContent="space-between">
              <FormControlLabel control={<Switch checked={d.sitemapEnabled} onChange={(e) => set({ sitemapEnabled: e.target.checked })} />} label="Serve sitemap.xml" />
              <Button variant="contained" disabled={busy || !liveProjectId} onClick={() => void save()}>Save metadata</Button>
            </Stack>
          </Stack>
        </Card>
      </Box>
    </Box>
  );
}
