"use client";

import {
  ArrowForwardRounded,
  AutoAwesomeRounded,
  CodeRounded,
  DataObjectRounded,
  RocketLaunchRounded,
  SecurityRounded,
} from "@mui/icons-material";
import { Box, Button, Card, Chip, Stack, Typography } from "@mui/material";
import Link from "next/link";
import type { ReactNode } from "react";
import { tokens } from "../theme";

export interface ReferencePageProps {
  eyebrow: string;
  title: string;
  description: string;
  primaryCta?: { label: string; href: string };
  secondaryCta?: { label: string; href: string };
  stats?: Array<[string, string]>;
  sections: Array<{
    title: string;
    body: string;
    icon: ReactNode;
  }>;
  workflow: Array<[string, string]>;
}

export function ReferencePage({
  eyebrow,
  title,
  description,
  primaryCta = { label: "Start a project", href: "/signup" },
  secondaryCta = { label: "Open templates", href: "/templates" },
  stats = [["92/100", "Gate score"], ["60 sec", "First setup"], ["1 flow", "Plan to deploy"]],
  sections,
  workflow,
}: ReferencePageProps) {
  return (
    <Box sx={{ mx: "auto", maxWidth: 1288, minWidth: 0 }}>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 1fr) minmax(0, .92fr)" },
          gap: { xs: 3, lg: 5 },
          alignItems: "center",
          py: { xs: 4, md: 7 },
          minWidth: 0,
        }}
      >
        <Stack spacing={2.4} sx={{ minWidth: 0 }}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ color: tokens.color.accent.violet }}>
            <AutoAwesomeRounded sx={{ fontSize: 18 }} />
            <Typography sx={{ fontSize: 12, fontWeight: 900, textTransform: "uppercase" }}>{eyebrow}</Typography>
          </Stack>
          <Typography
            component="h1"
            sx={{
              color: tokens.color.text.primary,
              fontSize: { xs: 38, md: 64 },
              fontWeight: 900,
              lineHeight: 1.04,
              letterSpacing: 0,
            }}
          >
            {title}
          </Typography>
          <Typography sx={{ color: tokens.color.text.secondary, fontSize: { xs: 15, md: 17 }, lineHeight: 1.65, maxWidth: 680 }}>
            {description}
          </Typography>
          <Stack direction={{ xs: "column", sm: "row" }} spacing={1.5}>
            <Button component={Link} href={primaryCta.href} variant="contained" endIcon={<ArrowForwardRounded sx={{ fontSize: 16 }} />}>
              {primaryCta.label}
            </Button>
            <Button component={Link} href={secondaryCta.href} sx={{ color: tokens.color.accent.violet }}>
              {secondaryCta.label}
            </Button>
          </Stack>
        </Stack>
        <ProductPreview stats={stats} />
      </Box>

      <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "repeat(3,minmax(0,1fr))" }, gap: 2, minWidth: 0 }}>
        {sections.map((section) => (
          <Card key={section.title} sx={{ p: 2.4, bgcolor: tokens.color.bg.surfaceRaised }}>
            <Box sx={{ color: tokens.color.accent.violet }}>{section.icon}</Box>
            <Typography sx={{ mt: 1.2, fontSize: 17, fontWeight: 900 }}>{section.title}</Typography>
            <Typography sx={{ mt: 0.8, color: tokens.color.text.secondary, fontSize: 13.5, lineHeight: 1.55 }}>{section.body}</Typography>
          </Card>
        ))}
      </Box>

      <Card
        sx={{
          mt: 3,
          p: { xs: 2.5, md: 3.5 },
          bgcolor: `${tokens.color.bg.surfaceRaised}d1`,
          borderColor: `${tokens.color.accent.violet}47`,
          overflow: "hidden",
        }}
      >
        <Typography sx={{ textAlign: "center", fontSize: { xs: 24, md: 30 }, fontWeight: 900 }}>
          One connected flow
        </Typography>
        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", md: "repeat(4,minmax(0,1fr))" }, gap: 1.5, mt: 3 }}>
          {workflow.map(([step, body], index) => (
            <Box key={step} sx={{ p: 1.8, borderRadius: 1, border: `1px solid ${tokens.color.border.subtle}`, bgcolor: tokens.color.bg.surface }}>
              <Chip
                size="small"
                label={`0${index + 1}`}
                sx={{ height: 22, bgcolor: `${tokens.color.accent.purple}33`, color: tokens.color.accent.violet }}
              />
              <Typography sx={{ mt: 1.2, fontWeight: 900 }}>{step}</Typography>
              <Typography sx={{ mt: 0.6, color: tokens.color.text.secondary, fontSize: 12.5, lineHeight: 1.5 }}>{body}</Typography>
            </Box>
          ))}
        </Box>
      </Card>
    </Box>
  );
}

function ProductPreview({ stats }: { stats: Array<[string, string]> }) {
  return (
    <Card
      sx={{
        p: 2,
        background: `linear-gradient(180deg, ${tokens.color.bg.surfaceRaised}eb, ${tokens.color.bg.inset}f5)`,
        borderColor: `${tokens.color.accent.violet}4d`,
        minWidth: 0,
      }}
    >
      <Stack direction="row" alignItems="center" spacing={1}>
        <Box sx={{ width: 28, height: 28, borderRadius: "50%", background: `linear-gradient(135deg, ${tokens.color.accent.coral}, ${tokens.color.accent.violet})` }} />
        <Box sx={{ flex: 1 }}>
          <Typography sx={{ fontWeight: 900 }}>IronFlyer Builder</Typography>
          <Typography sx={{ color: tokens.color.text.muted, fontSize: 11 }}>workspace/live-preview</Typography>
        </Box>
        <Chip size="small" label="Live" sx={{ bgcolor: `${tokens.color.accent.success}2e`, color: tokens.color.accent.success }} />
      </Stack>
      <Box sx={{ display: "grid", gridTemplateColumns: "repeat(3,minmax(0,1fr))", gap: 1, mt: 2 }}>
        {stats.map(([value, label]) => (
          <Box key={label} sx={{ p: 1.2, borderRadius: 1, border: `1px solid ${tokens.color.border.subtle}`, bgcolor: `${tokens.color.text.primary}0a` }}>
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 10.5 }}>{label}</Typography>
            <Typography sx={{ mt: 0.3, fontSize: { xs: 18, sm: 22 }, fontWeight: 900 }}>{value}</Typography>
          </Box>
        ))}
      </Box>
      <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "minmax(0,.9fr) minmax(0,1.1fr)" }, gap: 1.2, mt: 1.2 }}>
        <Box sx={{ p: 1.4, borderRadius: 1, bgcolor: tokens.color.bg.inset, border: `1px solid ${tokens.color.border.subtle}` }}>
          <Stack direction="row" alignItems="center" spacing={0.7}>
            <CodeRounded sx={{ color: tokens.color.accent.violet, fontSize: 16 }} />
            <Typography sx={{ color: tokens.color.text.secondary, fontSize: 11, fontFamily: tokens.font.mono }}>Dashboard.tsx</Typography>
          </Stack>
          <Box component="pre" sx={{ m: 0, mt: 1, color: tokens.color.accent.violet, fontFamily: tokens.font.mono, fontSize: 11, lineHeight: 1.7, whiteSpace: "pre-wrap" }}>
{`return (
  <ProductFlow>
    <Preview />
    <Deploy />
  </ProductFlow>
)`}
          </Box>
        </Box>
        <Box sx={{ p: 1.4, borderRadius: 1, bgcolor: `${tokens.color.text.primary}0a`, border: `1px solid ${tokens.color.border.subtle}` }}>
          <Stack spacing={1}>
            {[
              [<DataObjectRounded key="data" />, "Data models ready"],
              [<SecurityRounded key="security" />, "Auth and roles mapped"],
              [<RocketLaunchRounded key="deploy" />, "Deploy checklist live"],
            ].map(([icon, label]) => (
              <Stack key={String(label)} direction="row" alignItems="center" spacing={1}>
                <Box sx={{ color: tokens.color.accent.violet, "& svg": { fontSize: 16 } }}>{icon}</Box>
                <Typography sx={{ fontSize: 12.5, fontWeight: 800 }}>{label}</Typography>
              </Stack>
            ))}
          </Stack>
        </Box>
      </Box>
    </Card>
  );
}

