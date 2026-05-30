import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { Icon } from '../../icons';
import { TechIcon } from '../../lib/techIcons';

// One connector tile. Glass surface that "should not feel like a card":
// faint fill + hairline + blur, a soft accent wash under the rounded logo tile,
// a live gate-verdict chip when a finisher gate exists, and a connect control
// that flips to a success-neon "Connected" state. All values flow from the
// studio theme — zero raw color literals.
export type Connector = {
  name: string;
  desc: string;
  label: string;
  gate: string | null;
  glyph: string;
};

export function IntegrationCard(props: {
  connector: Connector;
  isOn: boolean;
  status?: string;
  onToggle: () => void;
}) {
  const { connector: c, isOn, status, onToggle } = props;
  const passed = status === 'passed';

  return (
    <Box
      sx={(theme) => ({
        position: 'relative',
        display: 'flex',
        flexDirection: 'column',
        p: 2.5,
        backgroundColor: theme.palette.cardBg,
        border: `1px solid ${isOn ? theme.studio.neon.success : theme.palette.cardBorder}`,
        borderRadius: `${theme.studio.effect.card.radius}px`,
        backdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        WebkitBackdropFilter: `blur(${theme.studio.effect.card.blur}px)`,
        overflow: 'hidden',
        transition: `transform ${theme.studio.motion.base}, border-color ${theme.studio.motion.base}, box-shadow ${theme.studio.motion.base}`,
        '&:hover': {
          transform: 'translateY(-3px)',
          borderColor: isOn ? theme.studio.neon.success : theme.palette.borderSubtle,
        },
        // Connected tiles carry a faint success rim-light at the top edge.
        ...(isOn && {
          boxShadow: `inset 0 1px 0 ${theme.studio.neon.success}33`,
        }),
      })}
    >
      <Stack direction="row" alignItems="flex-start" spacing={1.75} sx={{ mb: 2 }}>
        <Box
          aria-hidden
          sx={(theme) => ({
            display: 'grid',
            placeItems: 'center',
            width: 48,
            height: 48,
            flexShrink: 0,
            borderRadius: `${theme.studio.radius.sm}px`,
            color: theme.palette.text.primary,
            background: theme.studio.gradient.soft,
            border: `1px solid ${theme.palette.borderSubtle}`,
          })}
        >
          <TechIcon name={c.name} size={24} title={c.name} />
        </Box>

        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="subtitle1" sx={(theme) => ({ fontWeight: theme.typography.fontWeightBold, lineHeight: 1.15 })}>
            {c.name}
          </Typography>
          <Typography
            variant="caption"
            sx={(theme) => ({
              display: 'block',
              fontFamily: theme.brand.font.mono,
              color: theme.palette.text.disabled,
              letterSpacing: '0.02em',
            })}
          >
            {c.label} · {c.gate ?? 'no gate yet'}
          </Typography>
        </Box>

        {status ? (
          <Chip
            size="small"
            label={status}
            sx={(theme) => ({
              height: 22,
              fontWeight: theme.typography.fontWeightBold,
              textTransform: 'capitalize',
              color: passed ? theme.studio.neon.success : theme.studio.neon.warning,
              backgroundColor: 'transparent',
              border: `1px solid ${passed ? theme.studio.neon.success : theme.studio.neon.warning}55`,
              '& .MuiChip-label': { px: 1 },
            })}
          />
        ) : isOn ? (
          <Box
            aria-hidden
            sx={(theme) => ({
              display: 'grid',
              placeItems: 'center',
              width: 22,
              height: 22,
              borderRadius: theme.studio.radius.pill,
              color: theme.studio.neon.success,
              border: `1px solid ${theme.studio.neon.success}55`,
            })}
          >
            <Icon name="check" size={13} strokeWidth={2.5} />
          </Box>
        ) : null}
      </Stack>

      <Typography
        variant="body2"
        color="text.secondary"
        sx={{ mb: 2.5, minHeight: '2.75em', lineHeight: 1.45 }}
      >
        {c.desc}
      </Typography>

      <Button
        fullWidth
        variant={isOn ? 'outlined' : 'contained'}
        color={isOn ? 'inherit' : 'primary'}
        onClick={onToggle}
        startIcon={isOn ? <Icon name="check" size={16} strokeWidth={2.5} /> : <Icon name="add" size={16} strokeWidth={2.5} />}
        sx={(theme) => ({
          mt: 'auto',
          fontWeight: theme.typography.fontWeightMedium,
          ...(isOn && {
            color: theme.studio.neon.success,
            borderColor: `${theme.studio.neon.success}55`,
            '&:hover': {
              borderColor: theme.studio.neon.success,
              backgroundColor: `${theme.studio.neon.success}14`,
            },
          }),
        })}
      >
        {isOn ? 'Connected' : 'Connect'}
      </Button>
    </Box>
  );
}
