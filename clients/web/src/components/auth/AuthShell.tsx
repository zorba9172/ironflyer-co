// AuthShell — Base44-style split auth layout.
// (Pure presentation; renders as an RSC. The children slot (the form
// itself) is a client component, but the chrome around it is static.)
//
//   ┌─────────────────────────┬──────────────────────────┐
//   │ BRAND PANEL              │ FORM PANEL                │
//   │ (gates pitch, proof)     │ (title + form + switch)  │
//   └─────────────────────────┴──────────────────────────┘
//
// The page is full-bleed (no cockpit nav, no max-width wrapper). The
// left brand panel pitches Ironflyer's hard economic discipline; the
// right panel hosts the sign-in / sign-up form. Mobile collapses to a
// single column with the brand chrome reduced to a compact header.
//
// Every color is sourced from `tokens` or the MUI palette — never
// inline hex/rgba — per CLAUDE.md "Design reference is law".

import {
  ArrowBackRounded,
  AutoAwesomeRounded,
  BoltOutlined,
  CodeRounded,
  DataObjectRounded,
  RocketLaunchRounded,
  ShieldOutlined,
  TimelineOutlined,
} from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import Link from "next/link";
import type { ReactNode } from "react";
import { tokens } from "../../theme";
import { BrandLogo } from "../BrandLogo";

export type AuthMode = "signin" | "signup";

export interface AuthShellProps {
  mode: AuthMode;
  title: string;
  subtitle?: string;
  children: ReactNode;
  switchHref: string;
  switchPrompt: string;
  switchAction: string;
}

const PROOF_POINTS: Array<{
  icon: ReactNode;
  title: string;
  body: string;
}> = [
  {
    icon: <BoltOutlined sx={{ fontSize: 18 }} />,
    title: "Prompt to product",
    body: "Start from natural language and return to the same Studio workspace every time.",
  },
  {
    icon: <ShieldOutlined sx={{ fontSize: 18 }} />,
    title: "Built-in auth and roles",
    body: "Projects, access, approvals and launch flow stay connected to the account.",
  },
  {
    icon: <TimelineOutlined sx={{ fontSize: 18 }} />,
    title: "Preview and deploy",
    body: "Generated code, live previews and deploy checkpoints are waiting after sign-in.",
  },
];

export function AuthShell(props: AuthShellProps) {
  const { mode, title, subtitle, children, switchHref, switchPrompt, switchAction } = props;

  return (
    <Box
      sx={{
        position: "fixed",
        inset: 0,
        display: "grid",
        gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 1.05fr) minmax(0, 1fr)" },
        bgcolor: tokens.color.bg.base,
        color: tokens.color.text.primary,
        overflowY: "auto",
        overflowX: "clip",
      }}
    >
      {/* ── BRAND PANEL ─────────────────────────────────────────────── */}
      <Box
        sx={{
          position: "relative",
          display: { xs: "none", lg: "flex" },
          flexDirection: "column",
          justifyContent: "space-between",
          px: { lg: 6, xl: 8 },
          py: { lg: 5, xl: 6 },
          bgcolor: tokens.color.bg.surface,
          borderRight: `1px solid ${tokens.color.border.subtle}`,
          overflow: "hidden",
          minHeight: "100vh",
        }}
      >
        {/* violet wash glow — token-derived alpha, never raw rgba */}
        <Box
          aria-hidden
          sx={{
            position: "absolute",
            inset: 0,
            background: `radial-gradient(60% 45% at 15% 10%, ${tokens.color.accent.violet}22, transparent 70%), radial-gradient(50% 40% at 85% 95%, ${tokens.color.accent.coral}1f, transparent 70%)`,
            pointerEvents: "none",
          }}
        />

        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{ position: "relative", zIndex: 1 }}
        >
          <BrandLogo size={30} inverse href="/" />
          <Button
            component={Link}
            href="/"
            size="small"
            startIcon={<ArrowBackRounded sx={{ fontSize: 16 }} />}
            sx={{
              color: tokens.color.text.secondary,
              "&:hover": { color: tokens.color.text.primary, bgcolor: "transparent" },
            }}
          >
            Back to home
          </Button>
        </Stack>

        <Stack spacing={5} sx={{ position: "relative", zIndex: 1, maxWidth: 520 }}>
          <Box>
            <Box
              sx={{
                display: "inline-flex",
                alignItems: "center",
                gap: 0.75,
                px: 1.5,
                py: 0.5,
                borderRadius: `${tokens.radius.pill}px`,
                bgcolor: `${tokens.color.accent.violet}1a`,
                border: `1px solid ${tokens.color.border.subtle}`,
                color: tokens.color.accent.violet,
                fontFamily: tokens.font.mono,
                fontSize: 11,
                fontWeight: 700,
                letterSpacing: 1.2,
                textTransform: "uppercase",
              }}
            >
              AI app builder
            </Box>
            <Typography
              component="h1"
              sx={{
                mt: 3,
                fontSize: { lg: 40, xl: 48 },
                fontWeight: 800,
                lineHeight: 1.04,
                letterSpacing: -0.5,
                color: tokens.color.text.primary,
              }}
            >
              Continue your build,{" "}
              <Box
                component="span"
                sx={{
                  backgroundImage: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`,
                  WebkitBackgroundClip: "text",
                  WebkitTextFillColor: "transparent",
                }}
              >
                exactly where you left it.
              </Box>
            </Typography>
            <Typography
              sx={{
                mt: 2,
                fontSize: 15,
                lineHeight: 1.55,
                color: tokens.color.text.secondary,
                maxWidth: 460,
              }}
            >
              IronFlyer keeps the account experience inside the product:
              prompt, plan, code, preview, templates and deploy all return to
              the same workspace.
            </Typography>
          </Box>

          <StudioAuthPreview />
        </Stack>

        <Stack
          direction="row"
          spacing={2.5}
          sx={{
            position: "relative",
            zIndex: 1,
            pt: 3,
            borderTop: `1px solid ${tokens.color.border.subtle}`,
          }}
        >
          <ProofStat label="Plan status" value="Live" />
          <ProofStat label="Gate score" value="92" />
          <ProofStat label="Deploy lane" value="Ready" />
        </Stack>
      </Box>

      {/* ── FORM PANEL ──────────────────────────────────────────────── */}
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          minHeight: "100vh",
          bgcolor: tokens.color.bg.base,
        }}
      >
        {/* Mobile-only brand header (lg+ shows the brand panel) */}
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{
            display: { xs: "flex", lg: "none" },
            px: { xs: 2.5, sm: 4 },
            py: 2.5,
            borderBottom: `1px solid ${tokens.color.border.subtle}`,
          }}
        >
          <BrandLogo size={26} inverse href="/" />
          <Button
            component={Link}
            href="/"
            size="small"
            sx={{
              color: tokens.color.text.secondary,
              minWidth: 0,
              px: 1,
              "&:hover": { color: tokens.color.text.primary, bgcolor: "transparent" },
            }}
          >
            Home
          </Button>
        </Stack>

        <Box
          sx={{
            flex: 1,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            px: { xs: 2.5, sm: 4 },
            py: { xs: 5, sm: 6 },
          }}
        >
          <Box sx={{ width: "100%", maxWidth: 440 }}>
            <Stack spacing={3.5}>
              <Box>
                <Typography
                  component="h2"
                  sx={{
                    fontSize: { xs: 26, sm: 30 },
                    fontWeight: 800,
                    lineHeight: 1.15,
                    letterSpacing: -0.4,
                    color: tokens.color.text.primary,
                  }}
                >
                  {title}
                </Typography>
                {subtitle && (
                  <Typography
                    sx={{
                      mt: 1,
                      fontSize: 14,
                      lineHeight: 1.55,
                      color: tokens.color.text.secondary,
                    }}
                  >
                    {subtitle}
                  </Typography>
                )}
              </Box>

              <Box>{children}</Box>

              <Box
                sx={{
                  pt: 2.5,
                  borderTop: `1px solid ${tokens.color.border.subtle}`,
                  textAlign: "center",
                }}
              >
                <Typography sx={{ fontSize: 13, color: tokens.color.text.secondary }}>
                  {switchPrompt}{" "}
                  <Box
                    component={Link}
                    href={switchHref}
                    sx={{
                      color: tokens.color.accent.violet,
                      fontWeight: 700,
                      textDecoration: "none",
                      "&:hover": { textDecoration: "underline" },
                    }}
                  >
                    {switchAction} {mode === "signin" ? "→" : "←"}
                  </Box>
                </Typography>
              </Box>

              <Typography
                sx={{
                  fontSize: 11.5,
                  lineHeight: 1.55,
                  color: tokens.color.text.muted,
                  textAlign: "center",
                }}
              >
                By continuing you agree to use IronFlyer for lawful product
                builds and keep workspace activity connected to your account.
              </Typography>
            </Stack>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}

function StudioAuthPreview() {
  return (
    <Box
      sx={{
        borderRadius: `${tokens.radius.sm}px`,
        border: `1px solid ${tokens.color.border.strong}`,
        bgcolor: `${tokens.color.bg.inset}b8`,
        p: 1.6,
      }}
    >
      <Stack direction="row" alignItems="center" spacing={1}>
        <AutoAwesomeRounded sx={{ color: tokens.color.accent.violet, fontSize: 18 }} />
        <Typography sx={{ fontWeight: 900 }}>IronFlyer Studio</Typography>
        <Box sx={{ flex: 1 }} />
        <Box sx={{ px: 1, py: 0.35, borderRadius: `${tokens.radius.pill}px`, bgcolor: `${tokens.color.accent.success}29`, color: tokens.color.accent.success, fontSize: 11, fontWeight: 900 }}>
          Live
        </Box>
      </Stack>
      <Box sx={{ display: "grid", gridTemplateColumns: "minmax(0,.95fr) minmax(0,1.05fr)", gap: 1.2, mt: 1.4 }}>
        <Box sx={{ p: 1.2, borderRadius: `${tokens.radius.sm}px`, border: `1px solid ${tokens.color.border.subtle}`, bgcolor: tokens.color.bg.inset }}>
          <Stack direction="row" spacing={0.7} alignItems="center">
            <CodeRounded sx={{ color: tokens.color.accent.violet, fontSize: 15 }} />
            <Typography sx={{ color: tokens.color.text.muted, fontSize: 10.5, fontFamily: tokens.font.mono }}>
              App.tsx
            </Typography>
          </Stack>
          <Box component="pre" sx={{ m: 0, mt: 1, color: tokens.color.accent.violet, fontFamily: tokens.font.mono, fontSize: 10.5, lineHeight: 1.65, whiteSpace: "pre-wrap" }}>
{`return (
  <Portal>
    <Preview />
  </Portal>
)`}
          </Box>
        </Box>
        <Box sx={{ p: 1.2, borderRadius: `${tokens.radius.sm}px`, border: `1px solid ${tokens.color.border.subtle}`, bgcolor: `${tokens.color.text.primary}09` }}>
          {[
            [<DataObjectRounded key="data" />, "Data models ready"],
            [<ShieldOutlined key="roles" />, "Roles mapped"],
            [<RocketLaunchRounded key="deploy" />, "Deploy preview"],
          ].map(([icon, label]) => (
            <Stack key={String(label)} direction="row" spacing={0.8} alignItems="center" sx={{ py: 0.45 }}>
              <Box sx={{ color: tokens.color.accent.violet, "& svg": { fontSize: 15 } }}>{icon}</Box>
              <Typography sx={{ fontSize: 12.5, fontWeight: 800 }}>{label}</Typography>
            </Stack>
          ))}
        </Box>
      </Box>
      <Stack spacing={2.2} sx={{ mt: 2 }}>
        {PROOF_POINTS.map((p) => (
          <Stack key={p.title} direction="row" spacing={1.4} alignItems="flex-start">
            <Box
              sx={{
                flexShrink: 0,
                width: 30,
                height: 30,
                borderRadius: `${tokens.radius.sm}px`,
                bgcolor: tokens.color.bg.surfaceRaised,
                border: `1px solid ${tokens.color.border.subtle}`,
                color: tokens.color.accent.violet,
                display: "grid",
                placeItems: "center",
              }}
            >
              {p.icon}
            </Box>
            <Box>
              <Typography sx={{ fontWeight: 800, fontSize: 13.5, color: tokens.color.text.primary }}>
                {p.title}
              </Typography>
              <Typography sx={{ mt: 0.25, fontSize: 12.5, lineHeight: 1.45, color: tokens.color.text.secondary }}>
                {p.body}
              </Typography>
            </Box>
          </Stack>
        ))}
      </Stack>
    </Box>
  );
}

function ProofStat({ label, value }: { label: string; value: string }) {
  return (
    <Box sx={{ flex: 1 }}>
      <Typography
        sx={{
          fontFamily: tokens.font.mono,
          fontSize: 20,
          fontWeight: 700,
          color: tokens.color.text.primary,
          lineHeight: 1,
        }}
      >
        {value}
      </Typography>
      <Typography
        sx={{
          mt: 0.5,
          fontSize: 11,
          fontFamily: tokens.font.mono,
          fontWeight: 600,
          letterSpacing: 1.1,
          textTransform: "uppercase",
          color: tokens.color.text.muted,
        }}
      >
        {label}
      </Typography>
    </Box>
  );
}
