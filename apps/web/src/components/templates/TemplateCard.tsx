"use client";

// TemplateCard — single tile used by both /templates (the public
// gallery) and the home TemplatesGalleryPreview rail. Click anywhere
// on the card to land in Studio with the template slug pre-pinned via
// sessionStorage; the query string (?template=slug) carries the same
// signal so deep links work even with sessionStorage disabled.
//
// Two visual modes:
//   variant="standard" — full card with thumbnail (gradient placeholder),
//                        title, description, tag chips. Used in the grid.
//   variant="featured" — wide hero card with eyebrow + larger title.
//                        Used at the top of the gallery.

import { ArrowForwardRounded } from "@mui/icons-material";
import { Box, Chip, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { tokens } from "../../theme";
import {
  persistTemplatePick,
  type TemplateData,
} from "./templateData";

export type TemplateCardVariant = "standard" | "featured" | "compact";

export interface TemplateCardProps {
  template: TemplateData;
  variant?: TemplateCardVariant;
  eyebrow?: string;
  // Optional override — when provided the card invokes onPick instead
  // of navigating to /studio. Used by the home composer rail to seed
  // the prompt without leaving the page.
  onPick?: (template: TemplateData) => void;
}

function buildHref(slug: string): string {
  const usp = new URLSearchParams({ template: slug });
  return `/studio?${usp.toString()}`;
}

// Deterministic gradient placeholder per template. The thumbnail is a
// 16:10 box painted with a violet → purple wash plus a soft radial
// highlight; the offsets shift by slug hash so adjacent cards do not
// look identical in the grid.
function gradientFor(slug: string): string {
  let h = 0;
  for (let i = 0; i < slug.length; i += 1) {
    h = (h << 5) - h + slug.charCodeAt(i);
    h |= 0;
  }
  const a = Math.abs(h % 60);
  const b = 30 + Math.abs((h >> 4) % 50);
  const violet = tokens.color.accent.violet;
  const purple = tokens.color.accent.purple;
  return [
    `radial-gradient(circle at ${20 + a}% ${30 + (a % 30)}%, ${violet}55, transparent 60%)`,
    `radial-gradient(circle at ${70 + (b % 20)}% ${65 + (a % 25)}%, ${purple}40, transparent 55%)`,
    `linear-gradient(135deg, ${violet} 0%, ${purple} 100%)`,
  ].join(", ");
}

export function TemplateCard({
  template,
  variant = "standard",
  eyebrow,
  onPick,
}: TemplateCardProps) {
  const featured = variant === "featured";
  const compact = variant === "compact";

  // Click handler: persist the slug into sessionStorage so the Studio
  // composer can prefill the prompt. If onPick is provided (home
  // composer rail), invoke it and prevent navigation — the home page
  // owns the seed flow without leaving the route.
  const onActivate = (e: React.MouseEvent) => {
    persistTemplatePick({
      slug: template.slug,
      name: template.name,
      description: template.description,
    });
    if (onPick) {
      e.preventDefault();
      onPick(template);
    }
  };

  const linkProps = onPick
    ? { role: "button" as const }
    : { component: Link as never, href: buildHref(template.slug) };

  return (
    <Box
      {...linkProps}
      onClick={onActivate}
      aria-label={`Open template ${template.name} in Studio`}
      sx={{
        position: "relative",
        display: "flex",
        flexDirection: featured ? { xs: "column", md: "row" } : "column",
        gap: featured ? { xs: 2, md: 3 } : 0,
        bgcolor: tokens.color.bg.surface,
        border: `1px solid ${tokens.color.border.subtle}`,
        borderRadius: 1,
        overflow: "hidden",
        textDecoration: "none",
        color: "inherit",
        cursor: "pointer",
        transition: `border-color ${tokens.motion.fast} ${tokens.motion.snap}, transform ${tokens.motion.fast} ${tokens.motion.snap}, background-color ${tokens.motion.fast} ${tokens.motion.snap}`,
        minHeight: featured ? { xs: 320, md: 280 } : 0,
        width: compact ? 260 : "auto",
        flexShrink: compact ? 0 : 1,
        "&:hover": {
          borderColor: tokens.color.border.strong,
          bgcolor: tokens.color.bg.surfaceHover,
          transform: "translateY(-1px)",
          "& [data-template-cta]": {
            color: tokens.color.text.primary,
          },
        },
        "&:focus-visible": {
          outline: `2px solid ${tokens.color.accent.violet}`,
          outlineOffset: 2,
        },
      }}
    >
      <Box
        sx={{
          position: "relative",
          width: featured ? { xs: "100%", md: "44%" } : "100%",
          aspectRatio: compact ? "16 / 8" : "16 / 10",
          flexShrink: 0,
          background: gradientFor(template.slug),
          borderBottom: featured
            ? { xs: `1px solid ${tokens.color.border.subtle}`, md: "none" }
            : `1px solid ${tokens.color.border.subtle}`,
          borderRight: featured
            ? { xs: "none", md: `1px solid ${tokens.color.border.subtle}` }
            : "none",
        }}
      >
        {/* Subtle inner border so the thumbnail reads as engineered, not
            marketing — matches the cockpit's "card-on-card" treatment. */}
        <Box
          sx={{
            position: "absolute",
            inset: 8,
            borderRadius: 0.75,
            border: `1px solid ${tokens.color.text.primary}1f`,
            pointerEvents: "none",
          }}
        />
      </Box>

      <Stack
        spacing={1.5}
        sx={{
          flex: 1,
          minWidth: 0,
          p: featured ? { xs: 2.5, md: 3 } : { xs: 2, md: 2.5 },
        }}
      >
        {(featured || eyebrow) && (
          <Typography
            variant="overline"
            sx={{
              color: tokens.color.accent.violet,
              letterSpacing: 1.4,
              fontWeight: 800,
              fontSize: 10.5,
            }}
          >
            {eyebrow ?? "Featured this week"}
          </Typography>
        )}

        <Typography
          component="h3"
          sx={{
            fontSize: featured ? { xs: 22, md: 26 } : { xs: 16, md: 17 },
            fontWeight: featured ? 800 : 700,
            letterSpacing: -0.3,
            color: tokens.color.text.primary,
            lineHeight: 1.2,
          }}
        >
          {template.name}
        </Typography>

        <Typography
          sx={{
            color: tokens.color.text.secondary,
            fontSize: featured ? 14.5 : 13.5,
            lineHeight: 1.5,
            display: "-webkit-box",
            WebkitLineClamp: featured ? 3 : 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
          }}
        >
          {template.description}
        </Typography>

        <Stack
          direction="row"
          spacing={0.75}
          flexWrap="wrap"
          useFlexGap
          sx={{ rowGap: 0.75, mt: "auto" }}
        >
          {template.tags.map((t) => (
            <Chip
              key={t}
              label={t}
              size="small"
              sx={{
                bgcolor: tokens.color.bg.surfaceRaised,
                color: tokens.color.text.secondary,
                border: `1px solid ${tokens.color.border.subtle}`,
                fontFamily: tokens.font.mono,
                fontSize: 10.5,
                letterSpacing: 0.4,
                fontWeight: 700,
                height: 22,
                borderRadius: 0.75,
                textTransform: "lowercase",
                "& .MuiChip-label": { px: 1 },
              }}
            />
          ))}
        </Stack>

        <Stack
          direction="row"
          alignItems="center"
          spacing={0.5}
          data-template-cta
          sx={{
            color: tokens.color.accent.violet,
            fontSize: 13,
            fontWeight: 700,
            letterSpacing: 0.2,
            mt: 0.5,
            transition: `color ${tokens.motion.fast} ${tokens.motion.snap}`,
          }}
        >
          <Box component="span">Open template</Box>
          <ArrowForwardRounded sx={{ fontSize: 14 }} />
        </Stack>
      </Stack>
    </Box>
  );
}
