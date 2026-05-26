// PageHeader — title + optional eyebrow + optional breadcrumb + CTA.
// (Pure presentation; renders as an RSC. `actions` is a ReactNode so a
// page can still pass a client-only <Button onClick> inside without
// forcing the header itself into the client bundle.)
// Used at the top of every cockpit page so spacing and typography stay
// consistent across A47–A50 surfaces.

import { ChevronRightRounded } from "@mui/icons-material";
import { Box, Breadcrumbs, Stack, Typography, type SxProps, type Theme } from "@mui/material";
import Link from "next/link";
import type { ReactNode } from "react";
import { tokens } from "../../theme";

export interface BreadcrumbItem {
  label: string;
  href?: string;
}

export interface PageHeaderProps {
  title: ReactNode;
  eyebrow?: ReactNode;
  description?: ReactNode;
  breadcrumbs?: BreadcrumbItem[];
  actions?: ReactNode;
  sx?: SxProps<Theme>;
}

export function PageHeader({
  title,
  eyebrow,
  description,
  breadcrumbs,
  actions,
  sx,
}: PageHeaderProps) {
  return (
    <Box sx={{ mb: { xs: 3, md: 4 }, ...sx }}>
      {breadcrumbs && breadcrumbs.length > 0 && (
        <Breadcrumbs
          separator={<ChevronRightRounded sx={{ fontSize: 14, color: tokens.color.text.muted }} />}
          sx={{ mb: 1.5, fontSize: 12, color: tokens.color.text.secondary }}
        >
          {breadcrumbs.map((b, i) =>
            b.href ? (
              <Link
                key={i}
                href={b.href}
                style={{ color: tokens.color.text.secondary, textDecoration: "none" }}
              >
                {b.label}
              </Link>
            ) : (
              <Typography key={i} sx={{ fontSize: 12, color: tokens.color.text.primary }}>
                {b.label}
              </Typography>
            ),
          )}
        </Breadcrumbs>
      )}
      <Stack
        direction={{ xs: "column", md: "row" }}
        spacing={{ xs: 2, md: 3 }}
        alignItems={{ md: "flex-end" }}
        justifyContent="space-between"
      >
        <Box>
          {eyebrow && (
            <Typography
              variant="overline"
              sx={{ display: "block", color: tokens.color.accent.violet, mb: 0.5 }}
            >
              {eyebrow}
            </Typography>
          )}
          <Typography
            component="h1"
            sx={{
              fontSize: { xs: 28, md: 34 },
              fontWeight: 800,
              letterSpacing: -0.6,
              lineHeight: 1.1,
              color: tokens.color.text.primary,
            }}
          >
            {title}
          </Typography>
          {description && (
            <Typography
              sx={{
                mt: 1,
                color: tokens.color.text.secondary,
                maxWidth: 760,
                fontSize: 14.5,
              }}
            >
              {description}
            </Typography>
          )}
        </Box>
        {actions && (
          <Stack
            direction="row"
            spacing={1.25}
            useFlexGap
            flexWrap="wrap"
            sx={{ flexShrink: 0, rowGap: 1 }}
          >
            {actions}
          </Stack>
        )}
      </Stack>
    </Box>
  );
}
