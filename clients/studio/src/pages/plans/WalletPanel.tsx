import { Box, Button, Chip, Stack, Typography } from '@mui/material';
import { useTheme } from '@mui/material/styles';
import { Icon } from '../../icons';
import { StudioChart, donutOption } from '../../components/charts';
import type { Wallet } from '../../hooks/useEconomics';

// ─────────────────────────────────────────────────────────────────────────
// Live wallet top-up module. Viz-first: a donut mirrors held vs. available
// credits with the operator decision named in its center ("available"), so the
// first read is the wallet state — not a number buried in prose. To the right,
// quick top-up amounts with the recommended amount on the neon CTA. All copy,
// data, and handlers are passed in by the page; this component owns only
// presentation. No raw color/size literals.
// ─────────────────────────────────────────────────────────────────────────

export function WalletPanel({
  wallet,
  isLive,
  topUps,
  recommended,
  formatUSD,
  onTopUp,
}: {
  wallet: Wallet;
  isLive: boolean;
  topUps: number[];
  recommended: number;
  formatUSD: (amount: number, opts?: { cents?: boolean }) => string;
  onTopUp: (amount: number) => void;
}) {
  const theme = useTheme();

  const available = Math.max(0, wallet.availableUSD);
  const held = Math.max(0, wallet.holdUSD);
  const hasBalance = isLive && available + held > 0;

  const donut = donutOption(theme, {
    data: [
      { name: 'Available', value: available, color: theme.studio.neon.success },
      { name: 'On hold', value: held, color: theme.studio.neon.purple },
    ],
    centerLabel: hasBalance ? `available` : `connect`,
    centerColor: hasBalance ? theme.studio.neon.success : theme.palette.text.disabled,
    emptyLabel: 'No balance',
    radius: ['64%', '86%'],
  });

  return (
    <Box
      sx={(t) => ({
        p: { xs: 2.5, md: 3 },
        borderRadius: `${t.studio.effect.card.radius}px`,
        backgroundColor: t.palette.cardBg,
        border: `1px solid ${t.palette.cardBorder}`,
        backdropFilter: `blur(${t.studio.effect.card.blur}px)`,
        WebkitBackdropFilter: `blur(${t.studio.effect.card.blur}px)`,
      })}
    >
      <Stack
        direction={{ xs: 'column', md: 'row' }}
        spacing={{ xs: 3, md: 4 }}
        alignItems={{ xs: 'stretch', md: 'center' }}
      >
        {/* Coverage donut — the visual mirror of wallet state. */}
        <Box sx={{ position: 'relative', width: 132, height: 132, flexShrink: 0, mx: { xs: 'auto', md: 0 } }}>
          <StudioChart option={donut} height={132} />
        </Box>

        {/* Context + balance. */}
        <Stack spacing={1} sx={{ flex: 1, minWidth: 0 }}>
          <Stack direction="row" spacing={1} alignItems="center" sx={{ flexWrap: 'wrap', gap: 1 }}>
            <Box
              aria-hidden
              sx={(t) => ({
                display: 'inline-flex',
                alignItems: 'center',
                justifyContent: 'center',
                width: 30,
                height: 30,
                borderRadius: t.studio.radius.sm,
                color: t.studio.neon.blue,
                backgroundColor: `${t.studio.neon.blue}1F`,
              })}
            >
              <Icon name="wallet" size={16} strokeWidth={2} />
            </Box>
            <Typography variant="h6" sx={(t) => ({ fontWeight: t.typography.fontWeightBold })}>
              Build wallet
            </Typography>
            <Chip
              size="small"
              label={isLive ? 'live' : 'offline preview'}
              sx={(t) => ({
                height: 22,
                fontWeight: t.typography.fontWeightBold,
                letterSpacing: '0.04em',
                textTransform: 'uppercase',
                color: isLive ? t.studio.neon.success : t.palette.text.disabled,
                backgroundColor: isLive ? `${t.studio.neon.success}1F` : t.palette.action.hover,
                border: isLive ? `1px solid ${t.studio.neon.success}3D` : `1px solid ${t.palette.divider}`,
              })}
            />
          </Stack>

          <Stack direction="row" alignItems="baseline" spacing={1}>
            <Typography component="span" sx={(t) => ({ fontSize: '1.9rem', fontWeight: t.typography.fontWeightBold, lineHeight: 1.1, letterSpacing: '-0.02em' })}>
              {isLive ? formatUSD(available) : '—'}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              available now
            </Typography>
          </Stack>

          <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 460 }}>
            {isLive
              ? 'Paid runs reserve first, then debit exactly what they use. A 402 clears the moment the next reservation can be covered.'
              : 'Connect to see your live balance. Paid runs reserve first, then debit exactly what they use.'}
          </Typography>
        </Stack>

        {/* Quick top-ups. */}
        <Stack spacing={1.25} sx={{ flexShrink: 0, width: { xs: '100%', md: 'auto' } }}>
          <Typography
            variant="caption"
            sx={(t) => ({ color: t.palette.text.disabled, letterSpacing: '0.06em', textTransform: 'uppercase', fontWeight: t.typography.fontWeightBold })}
          >
            Quick top-up
          </Typography>
          <Stack direction="row" spacing={1} sx={{ flexWrap: 'wrap', gap: 1 }}>
            {topUps.map((amount) => {
              const isRecommended = amount === recommended;
              return (
                <Button
                  key={amount}
                  variant={isRecommended ? 'contained' : 'outlined'}
                  color={isRecommended ? 'primary' : 'inherit'}
                  startIcon={<Icon name="add" size={15} strokeWidth={2.25} />}
                  onClick={() => onTopUp(amount)}
                  sx={(t) =>
                    isRecommended
                      ? {}
                      : {
                          borderColor: t.palette.divider,
                          color: t.palette.text.primary,
                          '&:hover': { borderColor: t.studio.neon.blue, backgroundColor: t.palette.surfaceHover },
                        }
                  }
                >
                  {formatUSD(amount, { cents: false })}
                </Button>
              );
            })}
          </Stack>
        </Stack>
      </Stack>
    </Box>
  );
}
