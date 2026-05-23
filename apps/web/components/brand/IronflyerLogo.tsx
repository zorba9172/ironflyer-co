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

const ink = '#0d0e0f';
const alabaster = tokens.color.bg.alabaster;
const lime = tokens.color.accent.lime;

export function IronflyerMark({ size = 32, tone = 'light', labelled = false }: IronflyerMarkProps) {
  const tile = tone === 'dark' ? alabaster : ink;
  const rail = tone === 'dark' ? ink : lime;
  const gate = tone === 'dark' ? lime : alabaster;

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
      <rect x="4" y="4" width="56" height="56" rx="8" fill={tile} />
      <path d="M19 14h13c9 0 15 5 15 13 0 6-3 10-9 12l10 11H35L26 40h-3v10H12V14h7Z" fill={rail} />
      <path d="M23 23h12c3 0 5 2 5 5s-2 5-5 5H23V23Z" fill={tile} />
      <path d="M15 14h10v36H15V14Z" fill={rail} />
      <path d="M28 18h16v4H28V18Zm0 12h16v4H28v-4Zm0 12h16v4H28v-4Z" fill={gate} />
      <path d="M46 24l8 8-8 8v-6h-6v-4h6v-6Z" fill={gate} />
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
                letterSpacing: '0.12em',
              }}
            >
              Finish, then ship
            </Typography>
          )}
        </Box>
      )}
    </Stack>
  );
}
