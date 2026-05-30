import { Box } from '@mui/material';
import type { SxProps } from '@mui/material';
import { useThemeMode } from '../../theme';

export function AmbientBackdrop(props?: { sx?: SxProps }) {
  const { mode } = useThemeMode();

  return (
    <Box
      aria-hidden
      sx={[
        (theme) => ({
          position: 'absolute',
          inset: 0,
          zIndex: 0,
          pointerEvents: 'none',
          overflow: 'hidden',
          transition: `background ${theme.studio.motion.base}`,
          background: theme.studio.effect.ambient[mode],
          '&::after': {
            content: '""',
            position: 'absolute',
            inset: 0,
            backgroundImage: `linear-gradient(to right, ${theme.studio.effect.gridLine} 1px, transparent 1px), linear-gradient(to bottom, ${theme.studio.effect.gridLine} 1px, transparent 1px)`,
            backgroundSize: '88px 88px',
            opacity: mode === 'light' ? 0.4 : 1,
            maskImage: `linear-gradient(to bottom, transparent 0%, ${theme.palette.common.black} 18%, ${theme.palette.common.black} 78%, transparent 100%)`,
            WebkitMaskImage: `linear-gradient(to bottom, transparent 0%, ${theme.palette.common.black} 18%, ${theme.palette.common.black} 78%, transparent 100%)`,
            transition: `opacity ${theme.studio.motion.base}`,
          },
        }),
        ...(Array.isArray(props?.sx) ? props!.sx : props?.sx ? [props.sx] : []),
      ]}
    />
  );
}
