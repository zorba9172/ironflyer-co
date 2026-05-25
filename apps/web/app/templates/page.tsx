"use client";

// /templates — public blueprint gallery with cockpit-grade polish.
//
// The page sits inside CockpitFrame. The shell auto-suppresses the
// cockpit chrome on marketing routes via usePathname; /templates is
// treated as cockpit-visible (so authenticated operators see the nav),
// which keeps the surface flush for both audiences.
//
// Data flow:
//   1. Try useBlueprintsQuery (orchestrator catalogue). If it returns
//      rows, map them into TemplateData shape via the helper in
//      templateData.ts.
//   2. If empty / loading / error, fall back to FALLBACK_TEMPLATES so
//      the surface never reads blank.
//
// Picking a template:
//   - Click anywhere on the card → /studio?template=<slug> AND drop
//     the slug into sessionStorage so the Studio composer can prefill
//     even if the query string is stripped by a client-side redirect.

import { Box, Skeleton, Stack } from "@mui/material";
import { useMemo, useState } from "react";
import {
  EmptyState,
  ErrorPanel,
  PageHeader,
} from "../../src/components/cockpit";
import { TemplateCard } from "../../src/components/templates/TemplateCard";
import {
  FALLBACK_TEMPLATES,
  TEMPLATE_CATEGORIES,
  mapBlueprintCategoryToTemplateCategory,
  type TemplateCategory,
  type TemplateData,
} from "../../src/components/templates/templateData";
import { useBlueprintsQuery } from "../../src/lib/gql/__generated__";
import { tokens } from "../../src/theme";
import { ProjectsFilterChips } from "../../src/components/projects/ProjectsFilterChips";

// Reuse the projects filter-chip primitive — same visual language, same
// active-state styling. The generic chip group from /executions is
// also viable, but the projects one carries the right minHeight and
// 44px tap target out of the box.

type CategoryFilter = TemplateCategory | "all";

// Light wrapper so the projects chip component (typed against
// ProjectFilter) accepts our category union without complaint. Cast
// is sound — ProjectsFilterChips only treats the value as an opaque
// string identifier and renders the label we provide.
type AnyChipOption = {
  value: string;
  label: string;
  count?: number;
};

export default function TemplatesPage() {
  return <TemplatesView />;
}

function TemplatesView() {
  const [filter, setFilter] = useState<CategoryFilter>("all");

  const blueprintsQuery = useBlueprintsQuery({
    fetchPolicy: "cache-and-network",
  });

  // Map orchestrator blueprints into the cockpit TemplateData shape.
  // Falls back to the curated list when the catalogue is empty so the
  // gallery never reads blank during local dev or first paint.
  const templates = useMemo<TemplateData[]>(() => {
    const live = blueprintsQuery.data?.blueprints ?? [];
    if (live.length === 0) return FALLBACK_TEMPLATES;
    return live.map<TemplateData>((b, i) => ({
      slug: b.id,
      name: b.name,
      description: b.description,
      category: mapBlueprintCategoryToTemplateCategory(b.category),
      tags: b.category
        ? b.category.toLowerCase().split(/[\s,/]+/).filter(Boolean).slice(0, 4)
        : [],
      featured: i === 0,
    }));
  }, [blueprintsQuery.data]);

  const featured = useMemo(
    () => templates.find((t) => t.featured) ?? templates[0],
    [templates],
  );

  const filtered = useMemo(() => {
    if (filter === "all") return templates;
    return templates.filter((t) => t.category === filter);
  }, [templates, filter]);

  // Grid renders every template except the featured hero (when the
  // user is on "All") so the eye does not see the same card twice.
  const gridTemplates = useMemo(() => {
    if (filter === "all" && featured) {
      return filtered.filter((t) => t.slug !== featured.slug);
    }
    return filtered;
  }, [filter, filtered, featured]);

  const counts = useMemo(() => {
    const c: Record<CategoryFilter, number> = {
      all: templates.length,
      "internal-tools": 0,
      "customer-apps": 0,
      workflows: 0,
      dashboards: 0,
      "api-services": 0,
    };
    for (const t of templates) {
      c[t.category] = (c[t.category] || 0) + 1;
    }
    return c;
  }, [templates]);

  const filterOptions: AnyChipOption[] = TEMPLATE_CATEGORIES.map((c) => ({
    value: c.value,
    label: c.label,
    count: counts[c.value],
  }));

  const loading = blueprintsQuery.loading && !blueprintsQuery.data;
  const error =
    blueprintsQuery.error && templates.length === 0
      ? blueprintsQuery.error
      : null;

  return (
    <Box>
      <PageHeader
        title="Templates"
        eyebrow="blueprint gallery"
        description="Start from a finished blueprint. Each template is a real Ironflyer project — gates already wired, costs already shaped, and the Studio composer pre-pinned to the right plan when you open it."
      />

      <Box sx={{ mb: { xs: 2, md: 2.5 } }}>
        <ProjectsFilterChips
          // Cast — see comment on AnyChipOption above. The chip group
          // is value-shape-agnostic; we own the union locally.
          options={filterOptions as never}
          value={filter as never}
          onChange={(next) => setFilter(next as CategoryFilter)}
        />
      </Box>

      {loading ? (
        <TemplatesGallerySkeleton />
      ) : error ? (
        <ErrorPanel
          error={error}
          title="Could not load templates"
          onRetry={() => void blueprintsQuery.refetch()}
        />
      ) : templates.length === 0 ? (
        <EmptyState
          title="No templates yet"
          body="The blueprint catalogue is empty. Describe your build from the home composer instead."
          cta={{ label: "Open Studio", href: "/studio" }}
        />
      ) : filtered.length === 0 ? (
        <EmptyState
          title="No templates in this category"
          body="Switch to All to see the full catalogue."
          cta={{ label: "Show all", onClick: () => setFilter("all") }}
        />
      ) : (
        <Stack spacing={{ xs: 2.5, md: 3 }}>
          {filter === "all" && featured && (
            <TemplateCard
              template={featured}
              variant="featured"
              eyebrow="Featured this week"
            />
          )}
          <Box
            sx={{
              display: "grid",
              gap: { xs: 1.5, md: 2 },
              gridTemplateColumns: {
                xs: "1fr",
                md: "repeat(2, minmax(0, 1fr))",
                lg: "repeat(3, minmax(0, 1fr))",
                xl: "repeat(4, minmax(0, 1fr))",
              },
              minWidth: 0,
            }}
          >
            {gridTemplates.map((t) => (
              <TemplateCard key={t.slug} template={t} />
            ))}
          </Box>
        </Stack>
      )}
    </Box>
  );
}

// TemplatesGallerySkeleton — placeholders shaped like the real grid so
// the page does not jump when the first paint resolves.
function TemplatesGallerySkeleton() {
  return (
    <Stack spacing={{ xs: 2.5, md: 3 }}>
      <Box
        sx={{
          bgcolor: tokens.color.bg.surface,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 1,
          minHeight: { xs: 320, md: 280 },
          p: { xs: 2.5, md: 3 },
          display: "grid",
          gap: 2,
          gridTemplateColumns: { xs: "1fr", md: "44% 1fr" },
        }}
      >
        <Skeleton
          variant="rounded"
          height="100%"
          sx={{ bgcolor: tokens.color.bg.surfaceRaised, minHeight: 160 }}
        />
        <Stack spacing={1.5}>
          <Skeleton
            variant="text"
            width="40%"
            height={18}
            sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
          />
          <Skeleton
            variant="text"
            width="80%"
            height={28}
            sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
          />
          <Skeleton
            variant="text"
            width="90%"
            height={16}
            sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
          />
          <Skeleton
            variant="text"
            width="70%"
            height={16}
            sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
          />
        </Stack>
      </Box>
      <Box
        sx={{
          display: "grid",
          gap: { xs: 1.5, md: 2 },
          gridTemplateColumns: {
            xs: "1fr",
            md: "repeat(2, minmax(0, 1fr))",
            lg: "repeat(3, minmax(0, 1fr))",
            xl: "repeat(4, minmax(0, 1fr))",
          },
        }}
      >
        {Array.from({ length: 6 }).map((_, i) => (
          <Box
            key={i}
            sx={{
              bgcolor: tokens.color.bg.surface,
              border: `1px solid ${tokens.color.border.subtle}`,
              borderRadius: 1,
              overflow: "hidden",
            }}
          >
            <Skeleton
              variant="rectangular"
              sx={{
                width: "100%",
                aspectRatio: "16 / 10",
                bgcolor: tokens.color.bg.surfaceRaised,
              }}
            />
            <Stack spacing={1} sx={{ p: { xs: 2, md: 2.5 } }}>
              <Skeleton
                variant="text"
                width="70%"
                height={22}
                sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
              />
              <Skeleton
                variant="text"
                width="95%"
                height={14}
                sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
              />
              <Skeleton
                variant="text"
                width="85%"
                height={14}
                sx={{ bgcolor: tokens.color.bg.surfaceRaised }}
              />
            </Stack>
          </Box>
        ))}
      </Box>
    </Stack>
  );
}
