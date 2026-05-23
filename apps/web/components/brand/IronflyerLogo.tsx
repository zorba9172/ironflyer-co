import { Box, Stack, Typography } from '@mui/material';
import { tokens } from '@tokens';

type Tone = 'light' | 'dark';

interface IronflyerMarkProps {
  size?: number;
  tone?: Tone;
  labelled?: boolean;
}

interface IronflyerLogoProps extends IronflyerMarkProps {
  wordmark?: boolean;
  tagline?: boolean;
}

const ink = tokens.color.brand.graphite;
const alabaster = tokens.color.bg.alabaster;
const lime = tokens.color.accent.lime;
const cyan = tokens.color.accent.sky;

export function IronflyerMark({ size = 32, tone = 'light', labelled = false }: IronflyerMarkProps) {
  const tile = tone === 'dark' ? alabaster : ink;
  const rail = tone === 'dark' ? ink : lime;
  const gate = tone === 'dark' ? lime : cyan;
  const cut = tone === 'dark' ? alabaster : '#111820';

  return (
    <Box
      component="svg"
      viewBox="0 0 64 64"
      role={labelled ? 'img' : 'presentation'}
      aria-label={labelled ? 'Ironflyer mark' : undefined}
      focusable="false"
      sx={{
        width: size,
        height: size,
        display: 'block',
        flexShrink: 0,
      }}
    >
      <rect x="4" y="4" width="56" height="56" rx="10" fill={tile} />
      <path d="M17 13h30v8H27v7h18v8H27v15H17V13Z" fill={rail} />
      <path d="M36 27h10.5L55 35.5 46.5 44H36l8.5-8.5L36 27Z" fill={gate} />
      <path d="M13 43h26v8H13v-8Z" fill={rail} />
      <path d="M27 28h13v8H27v-8Z" fill={cut} opacity={tone === 'dark' ? 0.24 : 1} />
      <circle cx="49" cy="17" r="3" fill={gate} />
    </Box>
  );
}

export function IronflyerLogo({
  size = 32,
  tone = 'light',
  labelled = true,
  wordmark = true,
  tagline = false,
}: IronflyerLogoProps) {
  const textColor = tone === 'dark' ? alabaster : ink;
  const mutedColor = tone === 'dark' ? '#9c968a' : '#5b554b';

  return (
    <Stack
      direction="row"
      spacing={1.2}
      alignItems="center"
      aria-label={labelled ? 'Ironflyer' : undefined}
    >
      <IronflyerMark size={size} tone={tone} labelled={false} />
      {wordmark && (
        <Box sx={{ minWidth: 0 }}>
          <Typography
            component="span"
            sx={{
              display: 'block',
              fontFamily: tokens.font.display,
              fontSize: Math.max(18, Math.round(size * 0.72)),
              lineHeight: 0.9,
              textTransform: 'uppercase',
              color: textColor,
              letterSpacing: 0,
            }}
          >
            Ironflyer
          </Typography>
          {tagline && (
            <Typography
              component="span"
              sx={{
                display: 'block',
                mt: 0.45,
                color: mutedColor,
                fontFamily: tokens.font.mono,
                fontSize: 10,
                lineHeight: 1,
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
              }}
            >
              Finisher OS
            </Typography>
          )}
        </Box>
      )}
    </Stack>
  );
}
