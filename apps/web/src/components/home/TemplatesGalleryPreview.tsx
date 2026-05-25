"use client";

// TemplatesGalleryPreview — a horizontal rail of compact TemplateCards
// under the home hero composer. Each card seeds the prompt with
// "Start from <name>: " when clicked (the home page owns the seed
// flow; we do not navigate away).
//
// Source of truth for the cards: useBlueprintsQuery first, with a
// fallback to the static FALLBACK_TEMPLATES list. Either way the row
// renders the shared TemplateCard component (variant="compact") so
// the home rail and the /templates gallery look like they were cut
// from the same fabric — no separate hardcoded chip pattern.

import { Box, Stack, Typography } from "@mui/material";
import { useMemo } from "react";
import { useBlueprintsQuery } from "../../lib/gql/__generated__";
import { tokens } from "../../theme";
import { ErrorPanel } from "../cockpit/ErrorPanel";
import { TemplateCard } from "../templates/TemplateCard";
import {
  FALLBACK_TEMPLATES,
  mapBlueprintCategoryToTemplateCategory,
  type TemplateData,
} from "../templates/templateData";

export interface TemplatesGalleryPreviewProps {
  onPick: (seed: string) => void;
}

export function TemplatesGalleryPreview({ onPick }: TemplatesGalleryPreviewProps) {
  const { data, loading, error } = useBlueprintsQuery({
    fetchPolicy: "cache-and-network",
  });

  const templates = useMemo<TemplateData[]>(() => {
    const live = data?.blueprints ?? [];
    if (live.length === 0) return FALLBACK_TEMPLATES;
    return live.map<TemplateData>((b) => ({
      slug: b.id,
      name: b.name,
      description: b.description,
      category: mapBlueprintCategoryToTemplateCategory(b.category),
      tags: b.category
        ? b.category.toLowerCase().split(/[\s,/]+/).filter(Boolean).slice(0, 3)
        : [],
    }));
  }, [data]);

  return (
    <Stack spacing={1.5}>
      <Typography
        sx={{
          fontSize: 14,
          fontWeight: 700,
          letterSpacing: -0.1,
          color: tokens.color.text.primary,
        }}
      >
        Start from a proven template
      </Typography>

      {error && <ErrorPanel error={error} title="Could not load templates" />}

      <Box
        sx={{
          display: "flex",
          gap: { xs: 1.25, md: 1.5 },
          overflowX: "auto",
          // Hide the scrollbar but keep the surface horizontally
          // scrollable on touch devices.
          scrollbarWidth: "none",
          "&::-webkit-scrollbar": { display: "none" },
          // Bleed the rail to the edge of the composer so cards do
          // not clip awkwardly when the viewport is narrow.
          mx: { xs: -0.5, md: 0 },
          px: { xs: 0.5, md: 0 },
          pb: 1,
          minWidth: 0,
        }}
      >
        {loading && !data
          ? Array.from({ length: 6 }).map((_, i) => (
              <Box
                key={i}
                sx={{
                  flex: "0 0 260px",
                  height: 200,
                  borderRadius: 1,
                  bgcolor: tokens.color.bg.surface,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  opacity: 0.55,
                }}
              />
            ))
          : templates.map((tpl) => (
              <TemplateCard
                key={tpl.slug}
                template={tpl}
                variant="compact"
                onPick={(t) => onPick(`Start from ${t.name}: `)}
              />
            ))}
      </Box>
    </Stack>
  );
}

export default TemplatesGalleryPreview;
