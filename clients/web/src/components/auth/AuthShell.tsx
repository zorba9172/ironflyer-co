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
  timing?: "light" | "dark";
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
    body: "Your latest prompt and generated workspace are ready after sign-in.",
  },
  {
    icon: <ShieldOutlined sx={{ fontSize: 18 }} />,
    title: "Built-in auth and roles",
    body: "Projects, access and approvals stay tied to the account.",
  },
  {
    icon: <TimelineOutlined sx={{ fontSize: 18 }} />,
    title: "Preview and deploy",
    body: "Previews and deploy checkpoints continue in the same lane.",
  },
];

export function AuthShell(props: AuthShellProps) {
  const {
    mode,
    timing = "dark",
    title,
    subtitle,
    children,
    switchHref,
    switchPrompt,
    switchAction,
  } = props;
  const light = timing === "light";
  const c = {
    bg: light ? "#fbfaff" : tokens.color.bg.base,
    surface: light ? "#ffffff" : tokens.color.bg.surface,
    inset: light ? "#f7f2ff" : tokens.color.bg.inset,
    raised: light ? "#ffffff" : tokens.color.bg.surfaceRaised,
    hover: light ? "rgba(143,77,255,0.08)" : tokens.color.bg.surfaceHover,
    text: light ? "#090d3d" : tokens.color.text.primary,
    secondary: light ? "#59617d" : tokens.color.text.secondary,
    muted: light ? "#858ba5" : tokens.color.text.muted,
    border: light ? "rgba(18,22,55,0.10)" : tokens.color.border.subtle,
    strong: light ? "rgba(143,77,255,0.20)" : tokens.color.border.strong,
    glowA: light ? "rgba(188,92,255,0.16)" : `${tokens.color.accent.violet}22`,
    glowB: light ? "rgba(255,111,76,0.10)" : `${tokens.color.accent.coral}1f`,
  };

  return (
    <Box
      sx={{
        position: "fixed",
        inset: 0,
        display: "grid",
        gridTemplateColumns: {
          xs: "1fr",
          lg: "minmax(420px, 0.9fr) minmax(0, 1.1fr)",
        },
        bgcolor: c.bg,
        color: c.text,
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
          gap: { lg: 3, xl: 4 },
          px: { lg: 5, xl: 7 },
          py: { lg: 3.5, xl: 5 },
          bgcolor: c.surface,
          borderRight: `1px solid ${c.border}`,
          overflow: "hidden",
          height: "100dvh",
          minHeight: 0,
        }}
      >
        {/* violet wash glow — token-derived alpha, never raw rgba */}
        <Box
          aria-hidden
          sx={{
            position: "absolute",
            inset: 0,
            background: `radial-gradient(60% 45% at 15% 10%, ${c.glowA}, transparent 70%), radial-gradient(50% 40% at 85% 95%, ${c.glowB}, transparent 70%)`,
            pointerEvents: "none",
          }}
        />

        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{ position: "relative", zIndex: 1 }}
        >
          <BrandLogo size={30} inverse={!light} href={`/?theme=${timing}`} />
          <Button
            component={Link}
            href={`/?theme=${timing}`}
            size="small"
            startIcon={<ArrowBackRounded sx={{ fontSize: 16 }} />}
            sx={{
              color: c.secondary,
              "&:hover": { color: c.text, bgcolor: "transparent" },
            }}
          >
            Back to home
          </Button>
        </Stack>

        <Stack
          spacing={{ lg: 3, xl: 3.5 }}
          sx={{
            position: "relative",
            zIndex: 1,
            maxWidth: 500,
            flex: 1,
            minHeight: 0,
            justifyContent: "center",
          }}
        >
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
                border: `1px solid ${c.border}`,
                color: tokens.color.accent.violet,
                fontFamily: tokens.font.mono,
                fontSize: 11,
                fontWeight: 700,
                letterSpacing: 0,
                textTransform: "uppercase",
              }}
            >
              Workspace sign-in
            </Box>
            <Typography
              component="h1"
              sx={{
                mt: 2.4,
                fontSize: { lg: 34, xl: 42 },
                fontWeight: 800,
                lineHeight: 1.06,
                letterSpacing: 0,
                color: c.text,
              }}
            >
              Continue where{" "}
              <Box
                component="span"
                sx={{
                  backgroundImage: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`,
                  WebkitBackgroundClip: "text",
                  WebkitTextFillColor: "transparent",
                }}
              >
                your build left off.
              </Box>
            </Typography>
            <Typography
              sx={{
                mt: 1.6,
                fontSize: 15,
                lineHeight: 1.5,
                color: c.secondary,
                maxWidth: 430,
              }}
            >
              Prompt, plan, code, preview, templates and deploy stay attached to
              the same IronFlyer Studio workspace.
            </Typography>
          </Box>

          <StudioAuthPreview colors={c} />
        </Stack>

        <Stack
          direction="row"
          spacing={2.5}
          sx={{
            position: "relative",
            zIndex: 1,
            mt: "auto",
            pt: 2,
            borderTop: `1px solid ${c.border}`,
            "@media (max-height: 760px)": {
              display: "none",
            },
          }}
        >
          <ProofStat label="Plan status" value="Live" colors={c} />
          <ProofStat label="Gate score" value="92" colors={c} />
          <ProofStat label="Deploy lane" value="Ready" colors={c} />
        </Stack>
      </Box>

      {/* ── FORM PANEL ──────────────────────────────────────────────── */}
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          minHeight: "100vh",
          bgcolor: c.bg,
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
            borderBottom: `1px solid ${c.border}`,
          }}
        >
          <BrandLogo size={26} inverse={!light} href={`/?theme=${timing}`} />
          <Button
            component={Link}
            href={`/?theme=${timing}`}
            size="small"
            sx={{
              color: c.secondary,
              minWidth: 0,
              px: 1,
              "&:hover": { color: c.text, bgcolor: "transparent" },
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
                    letterSpacing: 0,
                    color: c.text,
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
                      color: c.secondary,
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
                  borderTop: `1px solid ${c.border}`,
                  textAlign: "center",
                }}
              >
                <Typography sx={{ fontSize: 13, color: c.secondary }}>
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
                  color: c.muted,
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

function StudioAuthPreview({
  colors,
}: {
  colors: {
    inset: string;
    raised: string;
    border: string;
    strong: string;
    text: string;
    secondary: string;
    muted: string;
  };
}) {
  return (
    <Box
      sx={{
        borderRadius: `${tokens.radius.sm}px`,
        border: `1px solid ${colors.strong}`,
        bgcolor: colors.inset,
        p: { lg: 1.35, xl: 1.6 },
        maxHeight: { lg: 330, xl: 380 },
        overflow: "hidden",
      }}
    >
      <Stack direction="row" alignItems="center" spacing={1}>
        <AutoAwesomeRounded
          sx={{ color: tokens.color.accent.violet, fontSize: 18 }}
        />
        <Typography sx={{ color: colors.text, fontWeight: 900 }}>
          IronFlyer Studio
        </Typography>
        <Box sx={{ flex: 1 }} />
        <Box
          sx={{
            px: 1,
            py: 0.35,
            borderRadius: `${tokens.radius.pill}px`,
            bgcolor: `${tokens.color.accent.success}29`,
            color: tokens.color.accent.success,
            fontSize: 11,
            fontWeight: 900,
          }}
        >
          Live
        </Box>
      </Stack>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: "minmax(0, 1.1fr) minmax(0, .9fr)",
          gap: 1.1,
          mt: 1.2,
        }}
      >
        <Box
          sx={{
            p: 1.15,
            borderRadius: `${tokens.radius.sm}px`,
            border: `1px solid ${colors.border}`,
            bgcolor: colors.inset,
          }}
        >
          <Stack direction="row" spacing={0.7} alignItems="center">
            <CodeRounded
              sx={{ color: tokens.color.accent.violet, fontSize: 15 }}
            />
            <Typography
              sx={{
                color: colors.muted,
                fontSize: 10.5,
                fontFamily: tokens.font.mono,
              }}
            >
              App.tsx
            </Typography>
          </Stack>
          <Box
            component="pre"
            sx={{
              m: 0,
              mt: 0.8,
              color: tokens.color.accent.violet,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              lineHeight: 1.55,
              whiteSpace: "pre-wrap",
            }}
          >
            {`return (
  <Portal>
    <Preview />
  </Portal>
)`}
          </Box>
        </Box>
        <Box
          sx={{
            p: 1.15,
            borderRadius: `${tokens.radius.sm}px`,
            border: `1px solid ${colors.border}`,
            bgcolor: colors.raised,
          }}
        >
          {[
            [<DataObjectRounded key="data" />, "Models"],
            [<ShieldOutlined key="roles" />, "Roles"],
            [<RocketLaunchRounded key="deploy" />, "Deploy"],
          ].map(([icon, label]) => (
            <Stack
              key={String(label)}
              direction="row"
              spacing={0.8}
              alignItems="center"
              sx={{ py: 0.35 }}
            >
              <Box
                sx={{
                  color: tokens.color.accent.violet,
                  "& svg": { fontSize: 15 },
                }}
              >
                {icon}
              </Box>
              <Typography
                sx={{ color: colors.text, fontSize: 12.5, fontWeight: 800 }}
              >
                {label}
              </Typography>
            </Stack>
          ))}
        </Box>
      </Box>
      <Stack spacing={1.15} sx={{ mt: 1.4 }}>
        {PROOF_POINTS.map((p) => (
          <Stack
            key={p.title}
            direction="row"
            spacing={1.1}
            alignItems="flex-start"
            sx={{
              "@media (max-height: 720px)": {
                alignItems: "center",
              },
            }}
          >
            <Box
              sx={{
                flexShrink: 0,
                width: 26,
                height: 26,
                borderRadius: `${tokens.radius.sm}px`,
                bgcolor: colors.raised,
                border: `1px solid ${colors.border}`,
                color: tokens.color.accent.violet,
                display: "grid",
                placeItems: "center",
              }}
            >
              {p.icon}
            </Box>
            <Box>
              <Typography
                sx={{ fontWeight: 800, fontSize: 12.8, color: colors.text }}
              >
                {p.title}
              </Typography>
              <Typography
                sx={{
                  mt: 0.15,
                  fontSize: 11.8,
                  lineHeight: 1.35,
                  color: colors.secondary,
                  "@media (max-height: 720px)": {
                    display: "none",
                  },
                }}
              >
                {p.body}
              </Typography>
            </Box>
          </Stack>
        ))}
      </Stack>
    </Box>
  );
}

function ProofStat({
  label,
  value,
  colors,
}: {
  label: string;
  value: string;
  colors: { text: string; muted: string };
}) {
  return (
    <Box sx={{ flex: 1 }}>
      <Typography
        sx={{
          fontFamily: tokens.font.mono,
          fontSize: 20,
          fontWeight: 700,
          color: colors.text,
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
          letterSpacing: 0,
          textTransform: "uppercase",
          color: colors.muted,
        }}
      >
        {label}
      </Typography>
    </Box>
  );
}
