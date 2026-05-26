// CtaBand — final CTA band shared across all marketing pages.
//
// Server component. Primary CTA uses <Button variant="contained"
// color="primary"> so the locked coral→magenta→purple gradient comes
// from the theme — never inlined.

import { ArrowForwardRounded } from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { tokens } from "../../theme";

export interface CtaBandProps {
  heading: string;
  sub: string;
  primary: { href: string; label: string };
  secondary?: { href: string; label: string };
  chips?: string[];
}

export function CtaBand({
  heading,
  sub,
  primary,
  secondary,
  chips,
}: CtaBandProps) {
  return (
    <Box
      component="section"
      sx={{
        py: { xs: 8, md: 12 },
        width: "100%",
      }}
    >
      <Box
        sx={{
          maxWidth: 1180,
          mx: "auto",
          width: "100%",
          minWidth: 0,
          position: "relative",
          borderRadius: `${tokens.radius.lg}px`,
          border: `1px solid ${tokens.color.border.accent}`,
          overflow: "hidden",
          px: { xs: 3, md: 8 },
          py: { xs: 6, md: 9 },
          textAlign: "center",
          background: `radial-gradient(circle at 50% -10%, ${tokens.color.accent.violet}33, transparent 60%), linear-gradient(180deg, ${tokens.color.bg.surfaceRaised}f2, ${tokens.color.bg.inset}f5)`,
        }}
      >
        <Stack spacing={2.4} alignItems="center">
          <Typography
            component="h2"
            sx={{
              fontSize: { xs: 28, md: 44 },
              fontWeight: 900,
              letterSpacing: -0.8,
              lineHeight: 1.06,
              color: tokens.color.text.primary,
              maxWidth: 820,
            }}
          >
            {heading}
          </Typography>
          <Typography
            sx={{
              fontSize: { xs: 15, md: 17 },
              lineHeight: 1.6,
              color: tokens.color.text.secondary,
              maxWidth: 680,
            }}
          >
            {sub}
          </Typography>
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={1.5}
            sx={{ pt: 1.5 }}
          >
            <Button
              component={Link}
              href={primary.href}
              variant="contained"
              color="primary"
              size="large"
              endIcon={<ArrowForwardRounded sx={{ fontSize: 18 }} />}
            >
              {primary.label}
            </Button>
            {secondary && (
              <Button
                component={Link}
                href={secondary.href}
                variant="text"
                size="large"
                sx={{ color: tokens.color.accent.violet }}
              >
                {secondary.label}
              </Button>
            )}
          </Stack>
          {chips && chips.length > 0 && (
            <Stack
              direction="row"
              spacing={1}
              flexWrap="wrap"
              useFlexGap
              justifyContent="center"
              sx={{ pt: 2 }}
            >
              {chips.map((chip) => (
                <Box
                  key={chip}
                  sx={{
                    px: 1.4,
                    py: 0.5,
                    borderRadius: 999,
                    border: `1px solid ${tokens.color.border.subtle}`,
                    bgcolor: `${tokens.color.bg.base}80`,
                    fontFamily: tokens.font.mono,
                    fontSize: 11,
                    letterSpacing: 0.4,
                    color: tokens.color.text.secondary,
                  }}
                >
                  {chip}
                </Box>
              ))}
            </Stack>
          )}
        </Stack>
      </Box>
    </Box>
  );
}
