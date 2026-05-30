import { useEffect, useMemo, useState } from 'react';
import { Box, Button, Chip, FormControlLabel, Stack, Switch, TextField, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from '@ironflyer/ui-web/fx';
import { useGraphQLQuery, useRequest, operations } from '@ironflyer/data';
import { useOperateProjectId } from '../hooks/useOperateProjectId';
import { PaneHeader } from '../components/operate/PaneHeader';
import { StudioChart, gaugeOption, type EChartsOption } from '../components/charts';
import { GlassPanel, StatCard, SectionHeader } from '../components/studio';
import { text } from '@ironflyer/design-tokens/brand';

interface SeoSettings { projectID: string; title: string; description: string; keywords: string[]; ogImageURL: string; twitterHandle: string; canonicalURL: string; robots: string; sitemapEnabled: boolean; updatedAt: string }
interface SeoCheck { key: string; label: string; passed: boolean; detail: string }
interface SeoAudit { score: number; checks: SeoCheck[] }

const SAMPLE_SETTINGS: SeoSettings = { projectID: '', title: 'TaskFlow — projects that ship', description: '', keywords: [], ogImageURL: '', twitterHandle: '', canonicalURL: '', robots: 'index,follow', sitemapEnabled: true, updatedAt: '' };
const SAMPLE_AUDIT: SeoAudit = { score: 43, checks: [
  { key: 'title', label: 'Title tag', passed: true, detail: 'Good length and keyword density.' },
  { key: 'description', label: 'Meta description', passed: false, detail: 'Missing — add a concise 160-char description.' },
  { key: 'keywords', label: 'Keywords', passed: false, detail: 'No keywords set. Add 3–5 primary terms.' },
  { key: 'og_image', label: 'Open Graph image', passed: false, detail: 'No og:image — links unfurl blank.' },
  { key: 'canonical', label: 'Canonical URL', passed: false, detail: 'Missing canonical URL.' },
  { key: 'sitemap', label: 'Sitemap', passed: true, detail: 'sitemap.xml served correctly.' },
  { key: 'robots', label: 'Robots policy', passed: true, detail: 'index,follow' },
] };

function CheckIcon({ passed }: { passed: boolean }) {
  if (passed) {
    return (
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="20 6 9 17 4 12" />
      </svg>
    );
  }
  return (
    <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="9" />
      <line x1="12" y1="8" x2="12" y2="12" />
      <line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
  );
}

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

  const passing = audit.checks.filter((c) => c.passed).length;
  const total = audit.checks.length;
  const scoreColor = audit.score >= 80 ? t.palette.success.main : audit.score >= 50 ? t.palette.warning.main : t.palette.error.main;
  const gauge = useMemo<EChartsOption>(() => gaugeOption(t, {
    value: audit.score,
    color: scoreColor,
    formatter: '{value}',
    radius: '100%',
  }), [audit.score, scoreColor, t]);

  return (
    <Box sx={{ flex: 1, height: '100%', overflowY: 'auto', bgcolor: 'background.default', p: 3 }}>
      <Box sx={{ maxWidth: 1080, mx: 'auto' }}>
        <PaneHeader title="Marketing" isLive={isLive} subtitle="SEO & social metadata for the deployed app" />

        {/* Score hero + audit checks side by side */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '260px 1fr' }, gap: 2, mb: 2.5, alignItems: 'stretch' }}>
          {/* Score card */}
          <GlassPanel pad={2.5} accent={scoreColor} sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
            <Typography
              sx={(th) => ({ fontFamily: th.brand.font.mono, fontSize: text.s66, textTransform: 'uppercase', letterSpacing: '0.12em', color: 'text.disabled', mb: 0.5, alignSelf: 'flex-start' })}
            >
              SEO score
            </Typography>
            <StudioChart option={gauge} height={160} />
            <Stack direction="row" alignItems="center" spacing={1} sx={{ mt: -1 }}>
              <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: scoreColor }} />
              <Typography sx={{ fontSize: text.s78, color: 'text.secondary' }}>{passing}/{total} checks passing</Typography>
            </Stack>
            <Box sx={{ mt: 1.5, display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1, width: '100%' }}>
              <Box sx={(th) => ({ borderRadius: `${th.studio.radius.sm}px`, p: 1.2, bgcolor: `${th.studio.neon.success}12`, textAlign: 'center' })}>
                <Typography sx={{ fontSize: text.s130, fontWeight: 800, color: 'success.main' }}>{passing}</Typography>
                <Typography sx={{ fontSize: text.s64, color: 'text.disabled' }}>Passing</Typography>
              </Box>
              <Box sx={(th) => ({ borderRadius: `${th.studio.radius.sm}px`, p: 1.2, bgcolor: `${th.palette.warning.main}12`, textAlign: 'center' })}>
                <Typography sx={{ fontSize: text.s130, fontWeight: 800, color: 'warning.main' }}>{total - passing}</Typography>
                <Typography sx={{ fontSize: text.s64, color: 'text.disabled' }}>Open</Typography>
              </Box>
            </Box>
          </GlassPanel>

          {/* Audit checks */}
          <GlassPanel pad={2.5}>
            <SectionHeader eyebrow="Audit" title="SEO checks" subtitle="Fix open items to increase your score and organic reach." />
            <Stack spacing={1}>
              {audit.checks.map((c) => (
                <Stack
                  key={c.key}
                  direction="row"
                  alignItems="flex-start"
                  spacing={1.5}
                  sx={(th) => ({
                    p: 1.25,
                    borderRadius: `${th.studio.radius.sm}px`,
                    bgcolor: c.passed ? `${th.palette.success.main}0a` : `${th.palette.warning.main}0a`,
                    border: `1px solid ${c.passed ? th.palette.success.main : th.palette.warning.main}22`,
                  })}
                >
                  <Box sx={{ color: c.passed ? 'success.main' : 'warning.main', mt: 0.2, flexShrink: 0 }}>
                    <CheckIcon passed={c.passed} />
                  </Box>
                  <Box sx={{ minWidth: 0 }}>
                    <Typography sx={{ fontSize: text.s82, fontWeight: 700 }}>{c.label}</Typography>
                    <Typography sx={{ fontSize: text.s74, color: 'text.secondary' }}>{c.detail}</Typography>
                  </Box>
                  <Chip
                    size="small"
                    label={c.passed ? 'pass' : 'fix'}
                    sx={(th) => ({
                      height: 20,
                      fontSize: text.s60,
                      textTransform: 'uppercase',
                      flexShrink: 0,
                      bgcolor: c.passed ? `${th.palette.success.main}22` : `${th.palette.warning.main}22`,
                      color: c.passed ? 'success.main' : 'warning.main',
                    })}
                  />
                </Stack>
              ))}
            </Stack>
          </GlassPanel>
        </Box>

        {/* Stat strip */}
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' }, gap: 1.5, mb: 2.5 }}>
          <StatCard label="Title length" value={`${d.title.length}/60`} hint={d.title.length >= 30 && d.title.length <= 60 ? 'Good' : 'Needs attention'} accent={t.studio.neon.blue} />
          <StatCard label="Description" value={`${d.description.length}/160`} hint={d.description.length >= 50 ? 'Good' : 'Too short'} accent={t.studio.neon.violet} />
          <StatCard label="Keywords" value={`${d.keywords.length}`} hint={d.keywords.length >= 3 ? 'Set' : 'Add keywords'} accent={t.studio.neon.pink} />
          <StatCard label="Sitemap" value={d.sitemapEnabled ? 'Enabled' : 'Off'} hint={d.sitemapEnabled ? 'Served at /sitemap.xml' : 'Enable for indexing'} accent={d.sitemapEnabled ? t.studio.neon.success : t.palette.warning.main} />
        </Box>

        {/* Metadata editor */}
        <GlassPanel pad={2.5}>
          <SectionHeader eyebrow="Metadata" title="SEO & social settings" actions={
            <Button variant="contained" disabled={busy || !liveProjectId} onClick={() => void save()}>Save metadata</Button>
          } />
          <Stack spacing={1.75}>
            <TextField size="small" label="Page title" value={d.title} onChange={(e) => set({ title: e.target.value })} helperText={`${d.title.length}/60 chars`} />
            <TextField size="small" label="Meta description" value={d.description} onChange={(e) => set({ description: e.target.value })} multiline minRows={2} helperText={`${d.description.length}/160 chars`} />
            <TextField size="small" label="Keywords (comma-separated)" value={d.keywords.join(', ')} onChange={(e) => set({ keywords: e.target.value.split(',').map((k) => k.trim()).filter(Boolean) })} />
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 1.5 }}>
              <TextField size="small" label="OG image URL" value={d.ogImageURL} onChange={(e) => set({ ogImageURL: e.target.value })} />
              <TextField size="small" label="Twitter handle" value={d.twitterHandle} onChange={(e) => set({ twitterHandle: e.target.value })} />
              <TextField size="small" label="Canonical URL" value={d.canonicalURL} onChange={(e) => set({ canonicalURL: e.target.value })} />
              <TextField size="small" label="Robots" value={d.robots} onChange={(e) => set({ robots: e.target.value })} />
            </Box>
            <Stack direction="row" alignItems="center">
              <FormControlLabel control={<Switch checked={d.sitemapEnabled} onChange={(e) => set({ sitemapEnabled: e.target.checked })} />} label="Serve sitemap.xml" />
            </Stack>
          </Stack>
        </GlassPanel>
      </Box>
    </Box>
  );
}
