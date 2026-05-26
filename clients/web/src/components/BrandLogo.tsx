// IronFlyer wordmark. Real inline SVG matching the private handoff:
// three forward gradient strokes and a crisp wordmark.
//
// Pure presentation — no hooks, refs, state, or browser APIs — so the
// component renders as a React Server Component and ships zero JS for
// the brand mark itself. (next/link works in RSC.)

import { Box, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { tokens } from "../../../../packages/design-tokens";

export interface BrandLogoProps {
  compact?: boolean;
  inverse?: boolean;
  size?: number;
  href?: string;
}

export function BrandLogo({
  compact = false,
  inverse = false,
  size = 40,
  href = "/",
}: BrandLogoProps) {
  const glyph = (
    <svg
      width={Math.round(size * 1.5)}
      height={size}
      viewBox="0 0 48 32"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
      style={{ display: "block" }}
    >
      <defs>
        <linearGradient id="ifly-mark-a" x1="3" x2="44" y1="28" y2="4" gradientUnits="userSpaceOnUse">
          <stop stopColor={tokens.color.accent.coral} />
          <stop offset=".52" stopColor={tokens.color.accent.violet} />
          <stop offset="1" stopColor={tokens.color.accent.purple} />
        </linearGradient>
      </defs>
      <path d="M4 24.5 15.8 7.5h8.8L12.8 24.5H4Z" fill="url(#ifly-mark-a)" />
      <path d="M18 24.5 29.8 7.5h8.8L26.8 24.5H18Z" fill="url(#ifly-mark-a)" opacity=".92" />
      <path d="M31.5 24.5 43.2 7.5h4.4L35.8 24.5h-4.3Z" fill="url(#ifly-mark-a)" opacity=".78" />
    </svg>
  );

  return (
    <Stack
      aria-label="IronFlyer home"
      component={Link}
      direction="row"
      href={href}
      sx={{
        alignItems: "center",
        color: "inherit",
        gap: 0.8,
        minHeight: 40,
        textDecoration: "none",
      }}
    >
      <Box
        sx={{
          display: "grid",
          height: size,
          placeItems: "center",
          width: Math.round(size * 1.5),
        }}
      >
        {glyph}
      </Box>
      {!compact && (
        <Typography
          component="span"
          sx={{
            color: inverse ? tokens.color.text.primary : tokens.color.text.inverse,
            fontSize: Math.round(size * 0.62),
            fontWeight: 900,
            letterSpacing: -0.35,
            lineHeight: 1,
          }}
        >
          IronFlyer
        </Typography>
      )}
    </Stack>
  );
}
