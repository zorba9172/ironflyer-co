import { Box, Chip, Stack, Typography } from '@mui/material';
import { Icon, BrandAsset, type BrandAssetName } from '../../icons';
import { GlassPanel } from '../../components/studio';
import { recentProjects, type StudioProject } from '../../studioData';

// One uniform preview card per recent build — a soft-bordered tile with a calm
// abstract app thumbnail (bound to the project's accent), the project name, its
// source, and a live status chip mirroring real deploy/gate state. Hover lifts.

type Tone = 'success' | 'warning' | 'danger' | 'neutral';

function statusFor(p: StudioProject): { label: string; tone: Tone } {
  if (p.deploy.status === 'production') return { label: 'Shipped', tone: 'success' };
  if (p.gates.some((g) => g.status === 'blocked')) return { label: 'Blocked', tone: 'danger' };
  if (p.deploy.status === 'preview') return { label: 'Preview', tone: 'warning' };
  return { label: 'In progress', tone: 'neutral' };
}

function toneColor(theme: import('@mui/material').Theme, tone: Tone): string {
  switch (tone) {
    case 'success': return theme.palette.success.main;
    case 'warning': return theme.palette.warning.main;
    case 'danger': return theme.palette.error.main;
    default: return theme.palette.primary.main;
  }
}

// A build's status maps to a premium 3D badge that floats over its thumbnail —
// a single tasteful brand moment per card (shipped → rocket, blocked → firewall).
const BADGE_FOR: Record<Tone, BrandAssetName> = {
  success: 'build.shipped',
  danger: 'build.blocked',
  warning: 'build.preview',
  neutral: 'build.progress',
};

// A calm faux app-preview: a header bar + two stat blocks + ghost rows, tinted
// with the project accent. No raw colors — every fill reads from the palette. A
// branded 3D status badge floats in the top-right corner.
function Thumb({ accent, badge }: { accent: string; badge: BrandAssetName }) {
  return (
    <Box
      aria-hidden
      sx={(theme) => ({
        position: 'relative',
        height: 92,
        borderRadius: `${theme.studio.radius.md}px`,
        border: `1px solid ${theme.palette.borderSubtle}`,
        bgcolor: theme.palette.surfaceHover,
        overflow: 'hidden',
        p: 1.25,
      })}
    >
      <Box
        sx={(theme) => ({
          position: 'absolute',
          top: 8,
          right: 8,
          width: 34,
          height: 34,
          borderRadius: `${theme.studio.radius.sm}px`,
          display: 'grid',
          placeItems: 'center',
          bgcolor: theme.palette.background.paper,
          border: `1px solid ${theme.palette.cardBorder}`,
          boxShadow: theme.brand.shadow.sm,
        })}
      >
        <BrandAsset name={badge} size={24} />
      </Box>
      <Stack direction="row" alignItems="center" spacing={0.75} sx={{ mb: 1 }}>
        <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: accent, flexShrink: 0 }} />
        <Box sx={(theme) => ({ width: 56, height: 6, borderRadius: 99, bgcolor: theme.palette.divider })} />
        <Box sx={{ flex: 1 }} />
        <Box sx={(theme) => ({ width: 20, height: 6, borderRadius: 99, bgcolor: theme.palette.divider })} />
      </Stack>
      <Stack direction="row" spacing={1} sx={{ mb: 1 }}>
        {[0, 1].map((i) => (
          <Box
            key={i}
            sx={(theme) => ({
              flex: 1,
              height: 32,
              borderRadius: `${theme.studio.radius.sm}px`,
              bgcolor: theme.palette.background.paper,
              border: `1px solid ${theme.palette.borderSubtle}`,
              boxShadow: i === 0 ? `inset 2px 0 0 ${accent}` : undefined,
            })}
          />
        ))}
      </Stack>
      <Stack spacing={0.6}>
        {[0.92, 0.7].map((w, i) => (
          <Box key={i} sx={(theme) => ({ width: `${w * 100}%`, height: 6, borderRadius: 99, bgcolor: theme.palette.divider })} />
        ))}
      </Stack>
    </Box>
  );
}

export function RecentBuilds(props: { onOpen?: (p: StudioProject) => void; onViewAll?: () => void }) {
  const builds = recentProjects.map((r) => r.project);
  // A small palette spread so the gallery reads as a friendly soft rainbow.
  const accents: Array<'primary' | 'cyan' | 'violet' | 'success'> = ['primary', 'cyan', 'violet', 'success'];

  return (
    <Box>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
        <Typography variant="h5" sx={{ fontWeight: 700 }}>Recent builds</Typography>
        <Stack
          direction="row"
          alignItems="center"
          spacing={0.5}
          onClick={props.onViewAll}
          sx={(theme) => ({
            cursor: 'pointer',
            color: theme.palette.text.secondary,
            transition: `color ${theme.studio.motion.fast}`,
            '&:hover': { color: theme.palette.primary.main },
          })}
        >
          <Typography variant="body2" sx={{ fontWeight: 600 }}>View all</Typography>
          <Icon name="arrowRight" size={15} />
        </Stack>
      </Stack>

      <Box
        sx={{
          display: 'grid',
          gap: 2,
          gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(4, 1fr)' },
        }}
      >
        {builds.map((p, i) => {
          const status = statusFor(p);
          const accentKey = accents[i % accents.length];
          return (
            <GlassPanel
              key={p.id}
              interactive
              pad={1.5}
              onClick={() => props.onOpen?.(p)}
              sx={(theme) => ({
                '--build-accent':
                  accentKey === 'cyan' ? theme.studio.neon.cyan
                  : accentKey === 'violet' ? theme.studio.neon.violet
                  : accentKey === 'success' ? theme.palette.success.main
                  : theme.palette.primary.main,
              })}
            >
              <Thumb accent="var(--build-accent)" badge={BADGE_FOR[status.tone]} />
              <Stack direction="row" alignItems="flex-start" justifyContent="space-between" spacing={1} sx={{ mt: 1.5 }}>
                <Box sx={{ minWidth: 0 }}>
                  <Typography variant="subtitle2" sx={{ fontWeight: 700 }} noWrap>{p.name}</Typography>
                  <Typography variant="caption" color="text.secondary" noWrap sx={{ display: 'block' }}>{p.source}</Typography>
                </Box>
                <Chip
                  size="small"
                  label={status.label}
                  sx={(theme) => {
                    const c = toneColor(theme, status.tone);
                    return {
                      flexShrink: 0,
                      height: 22,
                      fontWeight: 700,
                      color: c,
                      bgcolor: `${c}14`,
                      border: `1px solid ${c}33`,
                      '& .MuiChip-label': { px: 1 },
                    };
                  }}
                />
              </Stack>
            </GlassPanel>
          );
        })}
      </Box>
    </Box>
  );
}
