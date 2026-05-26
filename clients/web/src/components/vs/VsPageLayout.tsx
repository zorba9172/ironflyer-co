// Server component. Renders one /vs/[slug] comparison landing page
// from the typed Competitor record. Layout-only; no state, refs, or
// event handlers. All colors come from packages/design-tokens or the
// MUI theme palette per the constitutional design-reference rule.

import {
  ArrowForwardRounded,
  CheckRounded,
  CloseRounded,
  AutoAwesomeRounded,
  BoltRounded,
  ShieldOutlined,
  TrendingUpRounded,
} from "@mui/icons-material";
import {
  Box,
  Button,
  Card,
  Chip,
  Divider,
  Stack,
  Typography,
} from "@mui/material";
import Link from "next/link";
import { tokens } from "../../../../../packages/design-tokens";
import type { Competitor } from "./competitors";
import { competitors } from "./competitors";
import { buildFaq } from "./faq";

export function VsPageLayout({ competitor }: { competitor: Competitor }) {
  const faq = buildFaq(competitor);
  const related = competitors.filter((c) => c.slug !== competitor.slug);

  const subhead = `${competitor.name} ships a fast prompt-to-preview loop; Ironflyer adds the production discipline that turns a preview into a paid, gated, ledger-recorded execution.`;

  return (
    <Box
      sx={{
        mx: "auto",
        maxWidth: 1288,
        minWidth: 0,
        color: tokens.color.text.primary,
        px: { xs: 2, md: 0 },
      }}
    >
      {/* 1. Hero */}
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 1.05fr) minmax(0, 1fr)" },
          gap: { xs: 3, lg: 6 },
          alignItems: "center",
          py: { xs: 5, md: 8 },
        }}
      >
        <Stack spacing={2.4} sx={{ minWidth: 0 }}>
          <Stack direction="row" spacing={1.5} alignItems="center">
            <Chip
              label={competitor.category}
              size="small"
              sx={{
                bgcolor: `${tokens.color.accent.violet}1f`,
                color: tokens.color.accent.violet,
                fontWeight: 700,
                letterSpacing: 0.4,
                textTransform: "uppercase",
                fontSize: 11,
                border: `1px solid ${tokens.color.border.subtle}`,
              }}
            />
            <Stack direction="row" spacing={0.8} alignItems="center" sx={{ color: tokens.color.accent.violet }}>
              <AutoAwesomeRounded sx={{ fontSize: 16 }} />
              <Typography sx={{ fontSize: 11, fontWeight: 900, textTransform: "uppercase", letterSpacing: 0.6 }}>
                comparison
              </Typography>
            </Stack>
          </Stack>

          <Typography
            component="h1"
            sx={{
              color: tokens.color.text.primary,
              fontSize: { xs: 40, md: 64 },
              fontWeight: 900,
              lineHeight: 1.03,
              letterSpacing: -0.5,
            }}
          >
            Ironflyer vs {competitor.name}
          </Typography>

          <Typography sx={{ color: tokens.color.text.secondary, fontSize: { xs: 15, md: 18 }, lineHeight: 1.65, maxWidth: 680 }}>
            {subhead}
          </Typography>

          <Stack direction={{ xs: "column", sm: "row" }} spacing={1.5} sx={{ pt: 1 }}>
            <Button
              component={Link}
              href="/signup"
              variant="contained"
              color="primary"
              endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
            >
              Start free
            </Button>
            <Button
              component={Link}
              href="/product"
              sx={{ color: tokens.color.accent.violet, fontWeight: 700 }}
            >
              See product
            </Button>
          </Stack>
        </Stack>

        <Card
          sx={{
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: `${tokens.radius.lg}px`,
            p: { xs: 2.5, md: 3.5 },
            boxShadow: tokens.shadow.md,
          }}
        >
          <Stack spacing={2}>
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 11, fontWeight: 900, letterSpacing: 0.6, textTransform: "uppercase" }}>
              {competitor.name} tagline
            </Typography>
            <Typography sx={{ color: tokens.color.text.primary, fontSize: 18, fontWeight: 600, lineHeight: 1.45 }}>
              &ldquo;{competitor.tagline}&rdquo;
            </Typography>
            <Divider sx={{ borderColor: tokens.color.border.subtle }} />
            <Stack direction="row" spacing={1.5} alignItems="flex-start">
              <ShieldOutlined sx={{ fontSize: 20, color: tokens.color.accent.violet, mt: 0.4 }} />
              <Typography sx={{ color: tokens.color.text.secondary, fontSize: 14, lineHeight: 1.6 }}>
                Ironflyer keeps the prompt-to-preview speed and adds wallet enforcement, gate registry, patch review, real Docker workspaces, and a per-execution ledger so &ldquo;shipped&rdquo; is a verifiable verdict.
              </Typography>
            </Stack>
          </Stack>
        </Card>
      </Box>

      {/* 2. Honest scorecard */}
      <Box sx={{ py: { xs: 4, md: 6 } }}>
        <SectionHeader
          eyebrow="scorecard"
          title={`Six mechanics, side by side`}
          description={`Each row names a concrete production mechanic. Ironflyer ships it; ${competitor.name} does not as of 2026-05.`}
        />
        <Card
          sx={{
            mt: 3,
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: `${tokens.radius.lg}px`,
            overflow: "hidden",
          }}
        >
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: "minmax(0, 2fr) minmax(0, 1fr) minmax(0, 1fr)",
              bgcolor: tokens.color.bg.surfaceRaised,
              borderBottom: `1px solid ${tokens.color.border.subtle}`,
              px: { xs: 2, md: 3 },
              py: 1.6,
            }}
          >
            <HeaderCell label="Mechanic" />
            <HeaderCell label="Ironflyer" align="center" />
            <HeaderCell label={competitor.name} align="center" />
          </Box>
          {competitor.ironflyerWins.map((row, i) => (
            <Box
              key={row.mechanic}
              sx={{
                display: "grid",
                gridTemplateColumns: "minmax(0, 2fr) minmax(0, 1fr) minmax(0, 1fr)",
                px: { xs: 2, md: 3 },
                py: 1.8,
                borderBottom:
                  i < competitor.ironflyerWins.length - 1
                    ? `1px solid ${tokens.color.border.subtle}`
                    : "none",
                alignItems: "center",
              }}
            >
              <Stack spacing={0.4} sx={{ minWidth: 0 }}>
                <Typography sx={{ fontFamily: tokens.font.mono, color: tokens.color.text.primary, fontSize: 14, fontWeight: 700 }}>
                  {row.mechanic}
                </Typography>
                <Typography sx={{ color: tokens.color.text.muted, fontSize: 12.5, lineHeight: 1.5 }}>
                  {row.oneLineWhy}
                </Typography>
              </Stack>
              <Box sx={{ display: "flex", justifyContent: "center" }}>
                <CheckRounded sx={{ color: tokens.color.accent.success, fontSize: 22 }} />
              </Box>
              <Box sx={{ display: "flex", justifyContent: "center" }}>
                <CloseRounded sx={{ color: tokens.color.accent.danger, fontSize: 22 }} />
              </Box>
            </Box>
          ))}
        </Card>
      </Box>

      {/* 3. Where competitor is strong */}
      <Box sx={{ py: { xs: 4, md: 6 } }}>
        <SectionHeader
          eyebrow="credit where due"
          title={`Where ${competitor.name} is genuinely strong`}
          description={`A comparison page that pretends a competitor has zero strengths is useless. ${competitor.name} ships real value — here is what they do well today.`}
        />
        <Box
          sx={{
            mt: 3,
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
            gap: 2.5,
          }}
        >
          {competitor.whatTheyDoWell.map((point, i) => (
            <Card
              key={i}
              sx={{
                bgcolor: tokens.color.bg.surface,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: `${tokens.radius.lg}px`,
                p: 2.8,
                minHeight: 180,
              }}
            >
              <Stack spacing={1.5}>
                <Box
                  sx={{
                    width: 36,
                    height: 36,
                    borderRadius: `${tokens.radius.sm}px`,
                    bgcolor: i % 2 === 0 ? `${tokens.color.accent.sky}1f` : `${tokens.color.accent.violet}1f`,
                    color: i % 2 === 0 ? tokens.color.accent.sky : tokens.color.accent.violet,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                  }}
                >
                  <BoltRounded sx={{ fontSize: 18 }} />
                </Box>
                <Typography sx={{ color: tokens.color.text.primary, fontSize: 14.5, lineHeight: 1.6 }}>
                  {point}
                </Typography>
              </Stack>
            </Card>
          ))}
        </Box>
      </Box>

      {/* 4. Where competitor falls short */}
      <Box sx={{ py: { xs: 4, md: 6 } }}>
        <SectionHeader
          eyebrow="production gaps"
          title={`Where ${competitor.name} falls short for production`}
          description={`Five mechanic-specific gaps Ironflyer closes. Each card names the missing surface and the Ironflyer mechanism that fills it.`}
        />
        <Box
          sx={{
            mt: 3,
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", md: "repeat(3, 1fr)" },
            gap: 2.5,
          }}
        >
          {competitor.whereTheyFallShort.map((point, i) => (
            <Card
              key={i}
              sx={{
                bgcolor: tokens.color.bg.surface,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: `${tokens.radius.lg}px`,
                p: 2.8,
                minHeight: 200,
                position: "relative",
              }}
            >
              <Stack spacing={1.5}>
                <Stack direction="row" spacing={1.2} alignItems="center">
                  <Typography
                    sx={{
                      fontFamily: tokens.font.mono,
                      color: tokens.color.accent.coral,
                      fontSize: 12,
                      fontWeight: 700,
                      letterSpacing: 0.4,
                    }}
                  >
                    GAP {String(i + 1).padStart(2, "0")}
                  </Typography>
                </Stack>
                <Typography sx={{ color: tokens.color.text.primary, fontSize: 14.5, lineHeight: 1.6 }}>
                  {point}
                </Typography>
              </Stack>
            </Card>
          ))}
        </Box>
      </Box>

      {/* 5. Head-to-head wins */}
      <Box sx={{ py: { xs: 4, md: 6 } }}>
        <SectionHeader
          eyebrow="head-to-head"
          title="What Ironflyer ships that they don't"
          description="Six named mechanics with one-line proof. Each one is a real surface in the product — not a marketing label."
        />
        <Box
          sx={{
            mt: 3,
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)", lg: "repeat(3, 1fr)" },
            gap: 2.5,
          }}
        >
          {competitor.ironflyerWins.map((win) => (
            <Card
              key={win.mechanic}
              sx={{
                bgcolor: tokens.color.bg.surface,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: `${tokens.radius.lg}px`,
                p: 2.8,
                minHeight: 200,
              }}
            >
              <Stack spacing={1.5}>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    color: tokens.color.accent.violet,
                    fontSize: 14,
                    fontWeight: 700,
                    letterSpacing: 0.2,
                  }}
                >
                  {win.mechanic}
                </Typography>
                <Typography sx={{ color: tokens.color.text.secondary, fontSize: 14, lineHeight: 1.65 }}>
                  {win.oneLineWhy}
                </Typography>
                <Stack direction="row" spacing={0.8} alignItems="center" sx={{ color: tokens.color.accent.success }}>
                  <TrendingUpRounded sx={{ fontSize: 16 }} />
                  <Typography sx={{ fontSize: 11.5, fontWeight: 700, letterSpacing: 0.4, textTransform: "uppercase" }}>
                    shipped today
                  </Typography>
                </Stack>
              </Stack>
            </Card>
          ))}
        </Box>
      </Box>

      {/* 6. Switching from competitor */}
      <Box sx={{ py: { xs: 4, md: 6 } }}>
        <SectionHeader
          eyebrow="migration"
          title={`Switching from ${competitor.name}`}
          description="A short, honest path. No rewrite required for most projects."
        />
        <Card
          sx={{
            mt: 3,
            bgcolor: tokens.color.bg.surface,
            border: `1px solid ${tokens.color.border.subtle}`,
            borderRadius: `${tokens.radius.lg}px`,
            p: { xs: 2.5, md: 4 },
          }}
        >
          <Stack spacing={2.4} component="ol" sx={{ listStyle: "none", p: 0, m: 0 }}>
            {competitor.switchingNotes.map((note, i) => (
              <Stack
                key={i}
                component="li"
                direction="row"
                spacing={2}
                alignItems="flex-start"
              >
                <Box
                  sx={{
                    flexShrink: 0,
                    width: 32,
                    height: 32,
                    borderRadius: `${tokens.radius.sm}px`,
                    bgcolor: `${tokens.color.accent.violet}1f`,
                    color: tokens.color.accent.violet,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    fontFamily: tokens.font.mono,
                    fontSize: 13,
                    fontWeight: 700,
                  }}
                >
                  {String(i + 1).padStart(2, "0")}
                </Box>
                <Typography sx={{ color: tokens.color.text.primary, fontSize: 15, lineHeight: 1.65, pt: 0.6 }}>
                  {note}
                </Typography>
              </Stack>
            ))}
          </Stack>
        </Card>
      </Box>

      {/* 7. FAQ */}
      <Box sx={{ py: { xs: 4, md: 6 } }}>
        <SectionHeader
          eyebrow="faq"
          title="Common questions about switching"
          description="Migration, pricing, portability, model choice, and the 'just do the discipline yourself' question."
        />
        <Stack spacing={2} sx={{ mt: 3 }}>
          {faq.map((item) => (
            <Card
              key={item.q}
              sx={{
                bgcolor: tokens.color.bg.surface,
                border: `1px solid ${tokens.color.border.subtle}`,
                borderRadius: `${tokens.radius.lg}px`,
                p: { xs: 2.4, md: 3 },
              }}
            >
              <Stack spacing={1.2}>
                <Typography sx={{ color: tokens.color.text.primary, fontSize: 16, fontWeight: 700, lineHeight: 1.4 }}>
                  {item.q}
                </Typography>
                <Typography sx={{ color: tokens.color.text.secondary, fontSize: 14.5, lineHeight: 1.7 }}>
                  {item.a}
                </Typography>
              </Stack>
            </Card>
          ))}
        </Stack>
      </Box>

      {/* 8. CTA band */}
      <Box sx={{ py: { xs: 4, md: 7 } }}>
        <Card
          sx={{
            bgcolor: tokens.color.bg.surfaceRaised,
            border: `1px solid ${tokens.color.border.strong}`,
            borderRadius: `${tokens.radius.xl}px`,
            p: { xs: 3, md: 5 },
            display: "flex",
            flexDirection: { xs: "column", md: "row" },
            alignItems: { xs: "flex-start", md: "center" },
            justifyContent: "space-between",
            gap: 3,
            boxShadow: tokens.shadow.lg,
          }}
        >
          <Stack spacing={1.2} sx={{ maxWidth: 720 }}>
            <Typography
              component="h2"
              sx={{
                color: tokens.color.text.primary,
                fontSize: { xs: 24, md: 32 },
                fontWeight: 900,
                lineHeight: 1.15,
                letterSpacing: -0.3,
              }}
            >
              Ready to ship through gates, not vibes?
            </Typography>
            <Typography sx={{ color: tokens.color.text.secondary, fontSize: 15, lineHeight: 1.6 }}>
              Prepaid wallet, gate registry, real Docker workspaces, append-only ledger. One execution to feel the difference.
            </Typography>
          </Stack>
          <Stack direction={{ xs: "column", sm: "row" }} spacing={1.5}>
            <Button
              component={Link}
              href="/signup"
              variant="contained"
              color="primary"
              endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}
            >
              Start free
            </Button>
            <Button
              component={Link}
              href="/pricing"
              sx={{ color: tokens.color.accent.violet, fontWeight: 700 }}
            >
              See pricing
            </Button>
          </Stack>
        </Card>
      </Box>

      {/* 9. Related comparisons */}
      <Box sx={{ py: { xs: 3, md: 5 } }}>
        <Typography
          sx={{
            color: tokens.color.text.muted,
            fontSize: 11,
            fontWeight: 900,
            letterSpacing: 0.6,
            textTransform: "uppercase",
            mb: 2,
          }}
        >
          Related comparisons
        </Typography>
        <Stack direction="row" spacing={1.2} flexWrap="wrap" useFlexGap>
          {related.map((c) => (
            <Link
              key={c.slug}
              href={`/vs/${c.slug}`}
              style={{ textDecoration: "none" }}
            >
              <Chip
                clickable
                label={`Ironflyer vs ${c.name}`}
                sx={{
                  bgcolor: tokens.color.bg.surface,
                  color: tokens.color.text.primary,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  fontWeight: 600,
                  "&:hover": {
                    bgcolor: tokens.color.bg.surfaceHover,
                    borderColor: tokens.color.border.strong,
                  },
                }}
              />
            </Link>
          ))}
        </Stack>
      </Box>

      {/* Disclaimer */}
      <Box sx={{ py: 4 }}>
        <Typography sx={{ color: tokens.color.text.muted, fontSize: 12, lineHeight: 1.65, maxWidth: 920 }}>
          Comparison reflects public product information as of 2026-05. Features change; we will update this page when they do. Functional comparison, not legal claim.
        </Typography>
      </Box>
    </Box>
  );
}

function SectionHeader({
  eyebrow,
  title,
  description,
}: {
  eyebrow: string;
  title: string;
  description: string;
}) {
  return (
    <Stack spacing={1.2} sx={{ maxWidth: 820 }}>
      <Typography
        sx={{
          color: tokens.color.accent.violet,
          fontSize: 11,
          fontWeight: 900,
          letterSpacing: 0.6,
          textTransform: "uppercase",
        }}
      >
        {eyebrow}
      </Typography>
      <Typography
        component="h2"
        sx={{
          color: tokens.color.text.primary,
          fontSize: { xs: 26, md: 36 },
          fontWeight: 900,
          lineHeight: 1.15,
          letterSpacing: -0.3,
        }}
      >
        {title}
      </Typography>
      <Typography sx={{ color: tokens.color.text.secondary, fontSize: 15, lineHeight: 1.65 }}>
        {description}
      </Typography>
    </Stack>
  );
}

function HeaderCell({ label, align = "left" }: { label: string; align?: "left" | "center" }) {
  return (
    <Typography
      sx={{
        color: tokens.color.text.muted,
        fontSize: 11,
        fontWeight: 900,
        letterSpacing: 0.6,
        textTransform: "uppercase",
        textAlign: align,
      }}
    >
      {label}
    </Typography>
  );
}
