import { Box, Stack, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { StudioChart, donutOption } from '../../components/charts';
import { BUCKET_LABEL, BUCKET_ORDER, bucketColor, type StatusBucket } from './projectStatus';

// ─────────────────────────────────────────────────────────────────────────
// PortfolioStrip — a glanceable mirror of the whole portfolio (viz-first law):
// a compact donut whose center names the open decision ("building"), flanked by
// one stat chip per status bucket. Single glass panel, no nested cards. Colors
// and effects come entirely from the studio theme.
// ─────────────────────────────────────────────────────────────────────────

export function PortfolioStrip(props: { counts: Record<StatusBucket, number>; total: number }) {
  const { counts, total } = props;
  const theme = useTheme();

  const data = BUCKET_ORDER.filter((b) => counts[b] > 0).map((b) => ({
    name: BUCKET_LABEL[b],
    value: counts[b],
    color: bucketColor(theme, b),
  }));

  const building = counts.building;

  return (
    <Box
      sx={(t) => ({
        display: 'flex',
        flexDirection: { xs: 'column', sm: 'row' },
        alignItems: { xs: 'stretch', sm: 'center' },
        gap: { xs: 2, sm: 4 },
        p: { xs: 2.5, sm: 3 },
        backgroundColor: t.palette.cardBg,
        border: `1px solid ${t.palette.cardBorder}`,
        borderRadius: `${t.studio.effect.card.radius}px`,
        backdropFilter: `blur(${t.studio.effect.card.blur}px)`,
        WebkitBackdropFilter: `blur(${t.studio.effect.card.blur}px)`,
      })}
    >
      <Box sx={{ width: 132, height: 132, flexShrink: 0, mx: { xs: 'auto', sm: 0 } }}>
        <StudioChart
          option={donutOption(theme, {
            data,
            centerLabel: `${building}\nbuilding`,
            centerColor: theme.studio.neon.blue,
            emptyLabel: 'No projects',
            radius: ['62%', '84%'],
          })}
          height={132}
        />
      </Box>

      <Stack spacing={0.5} sx={{ flexShrink: 0 }}>
        <Typography
          variant="overline"
          sx={(t) => ({ color: t.palette.text.disabled, letterSpacing: '0.12em', lineHeight: 1.4 })}
        >
          Portfolio
        </Typography>
        <Typography variant="h3" sx={{ fontWeight: 800, lineHeight: 1 }}>
          {total}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {total === 1 ? 'project' : 'projects'} tracked
        </Typography>
      </Stack>

      <Stack
        direction="row"
        flexWrap="wrap"
        useFlexGap
        spacing={1.25}
        sx={{ flex: 1, justifyContent: { xs: 'center', sm: 'flex-end' } }}
      >
        {BUCKET_ORDER.map((b) => (
          <Stack
            key={b}
            direction="row"
            alignItems="center"
            spacing={1.25}
            sx={(t) => ({
              px: 2,
              py: 1.25,
              borderRadius: `${t.studio.radius.sm}px`,
              border: `1px solid ${t.palette.divider}`,
              backgroundColor: t.palette.surfaceRaised,
              minWidth: 110,
            })}
          >
            <Box sx={(t) => ({ width: 9, height: 9, borderRadius: t.studio.radius.pill, backgroundColor: bucketColor(t, b), flexShrink: 0 })} />
            <Box>
              <Typography variant="h6" sx={{ fontWeight: 700, lineHeight: 1.1 }}>
                {counts[b]}
              </Typography>
              <Typography variant="caption" sx={(t) => ({ color: t.palette.text.disabled, textTransform: 'uppercase', letterSpacing: '0.08em' })}>
                {BUCKET_LABEL[b]}
              </Typography>
            </Box>
          </Stack>
        ))}
      </Stack>
    </Box>
  );
}
