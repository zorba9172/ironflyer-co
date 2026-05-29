import { Box, Stack, Tooltip, Typography } from '@mui/material';
import { motion } from '@ironflyer/ui-web/fx';
import { formatUSD } from '@ironflyer/core';
import { useWallet, useSentinelForecast } from '../hooks/useEconomics';
import { useLiveProjectId } from '../hooks/useLiveProjectId';
import { useStudio } from '../store';

// Always-visible cost HUD: prepaid wallet balance + live burn rate, glanceable
// while you work. The category's #1 complaint is the surprise bill — no
// competitor surfaces live spend at all times. This is the moat made legible:
// you always see what you have and how fast it's going.
export function CostHUD() {
  const storeProjectId = useStudio((s) => s.liveProjectId);
  const firstProjectId = useLiveProjectId();
  const projectId = storeProjectId ?? firstProjectId;
  const { wallet, isLive } = useWallet();
  const { forecast } = useSentinelForecast(projectId);

  const burning = forecast.burnRatePerHourUSD > 0;
  const overHeadroom = forecast.remainingHeadroomUSD < 0 || wallet.availableUSD <= 0;
  const tight = !overHeadroom && (forecast.level === 'warn' || forecast.level === 'critical' || forecast.remainingHeadroomUSD < 1);
  const tone: 'error' | 'warning' | 'success' = overHeadroom ? 'error' : tight ? 'warning' : 'success';

  const tip = overHeadroom
    ? 'Wallet exhausted — top up before the next paid execution (402).'
    : burning
      ? `Burning ${formatUSD(forecast.burnRatePerHourUSD)}/h · projected ${formatUSD(forecast.extrapolatedTotalUSD)} · ${formatUSD(forecast.remainingHeadroomUSD)} headroom`
      : `Prepaid wallet · ${formatUSD(wallet.holdUSD)} held · lifetime spend ${formatUSD(wallet.lifetimeSpendUSD)}`;

  return (
    <Tooltip title={isLive ? tip : 'Connect to see live wallet + burn'} arrow>
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        sx={(t) => ({
          px: 1.25, py: 0.5, borderRadius: 99,
          border: 1, borderColor: 'divider', bgcolor: 'action.hover',
          cursor: 'default', userSelect: 'none',
          color: t.palette[tone].main,
        })}
      >
        <Box sx={{ position: 'relative', width: 8, height: 8, flexShrink: 0 }}>
          <Box sx={{ width: 8, height: 8, borderRadius: 99, bgcolor: 'currentColor' }} />
          {burning && !overHeadroom && (
            <Box
              component={motion.span}
              animate={{ scale: [1, 2.4], opacity: [0.55, 0] }}
              transition={{ duration: 1.4, repeat: Infinity, ease: 'easeOut' }}
              sx={{ position: 'absolute', inset: 0, borderRadius: 99, bgcolor: 'currentColor' }}
            />
          )}
        </Box>
        <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.74rem', fontWeight: 600, color: 'text.primary' })}>
          {formatUSD(wallet.availableUSD)}
        </Typography>
        {burning && (
          <Typography sx={(t) => ({ fontFamily: t.brand.font.mono, fontSize: '0.7rem', color: t.palette[tone].main })}>
            {formatUSD(forecast.burnRatePerHourUSD)}/h
          </Typography>
        )}
      </Stack>
    </Tooltip>
  );
}
