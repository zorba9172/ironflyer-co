import {
  CheckCircleRounded,
  RocketLaunchRounded,
  AutoAwesomeRounded,
  DesignServicesRounded,
  TerminalRounded,
} from "@mui/icons-material";
import { Box, Stack, Typography } from "@mui/material";
import { tokens } from "../../theme";

const tabs = ["Plan", "Build", "Review", "Deploy"] as const;

const workCards = [
  {
    label: "Brief",
    title: "Business intent parsed",
    body: "Audience, roles, screens and flows are locked before code starts.",
    icon: <AutoAwesomeRounded />,
  },
  {
    label: "Design",
    title: "Responsive UI system",
    body: "Components, tokens and conversion copy arrive as one product block.",
    icon: <DesignServicesRounded />,
  },
  {
    label: "Code",
    title: "React app generated",
    body: "Routes, data models, auth gates and integrations are mapped into the workspace.",
    icon: <TerminalRounded />,
  },
] as const;

const checklist = [
  "Product plan ready",
  "Mobile states generated",
  "Deploy checklist tracking",
] as const;

export function ProductTheater() {
  return (
    <Box
      sx={{
        position: "relative",
        minHeight: { xs: 430, md: 530 },
        borderRadius: 2,
        overflow: "hidden",
        border: `1px solid ${tokens.color.border.strong}`,
        bgcolor: `${tokens.color.bg.surfaceRaised}d9`,
        boxShadow: `0 26px 90px ${tokens.color.accent.purple}33`,
      }}
    >
      <Box
        sx={{
          position: "absolute",
          inset: 0,
          backgroundImage: [
            "url('/market/data-flow.jpg')",
            `radial-gradient(ellipse 360px 240px at 88% 8%, ${tokens.color.accent.violet}24, transparent 72%)`,
            `linear-gradient(145deg, ${tokens.color.bg.surfaceRaised}f2, ${tokens.color.bg.inset}f5)`,
          ].join(", "),
          backgroundSize: "cover, auto, auto",
          backgroundPosition: "center right, center, center",
          opacity: 0.9,
          mixBlendMode: "screen",
        }}
      />
      <Box
        sx={{
          position: "absolute",
          inset: 0,
          background: [
            `linear-gradient(180deg, ${tokens.color.bg.surfaceRaised}d4, ${tokens.color.bg.inset}f4 72%)`,
            `radial-gradient(circle at 1px 1px, ${tokens.color.text.primary}12 1px, transparent 1.5px)`,
          ].join(", "),
          backgroundSize: "auto, 28px 28px",
        }}
      />
      <Box sx={{ position: "relative", p: { xs: 2, md: 2.45 }, height: "100%" }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2.2 }}>
          <Stack direction="row" spacing={1} alignItems="center">
            <Box
              sx={{
                width: 26,
                height: 26,
                borderRadius: "50%",
                background: `linear-gradient(135deg, ${tokens.color.accent.coral}, ${tokens.color.accent.violet})`,
                boxShadow: `0 0 28px ${tokens.color.accent.violet}66`,
              }}
            />
            <Box>
              <Typography sx={{ fontSize: 14.5, fontWeight: 900, color: tokens.color.text.primary, lineHeight: 1.05 }}>
                IronFlyer Builder
              </Typography>
              <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 10.5, color: tokens.color.text.muted }}>
                workspace preview
              </Typography>
            </Box>
          </Stack>
          <Typography
            sx={{
              px: 1.3,
              py: 0.7,
              borderRadius: 1,
              bgcolor: `${tokens.color.accent.purple}1f`,
              border: `1px solid ${tokens.color.border.subtle}`,
              fontSize: 11,
              fontWeight: 800,
              color: tokens.color.text.primary,
            }}
          >
            Preview live
          </Typography>
        </Stack>

        <Box
          sx={{
            p: { xs: 1.6, md: 2 },
            borderRadius: 1.5,
            border: `1px solid ${tokens.color.border.subtle}`,
            bgcolor: `${tokens.color.bg.inset}e8`,
            boxShadow: `inset 0 1px 0 ${tokens.color.text.primary}0f`,
          }}
        >
          <Typography
            sx={{
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 0.7,
              textTransform: "uppercase",
              color: tokens.color.text.muted,
            }}
          >
            Prompt workspace
          </Typography>
          <Typography sx={{ mt: 0.8, fontSize: { xs: 17, md: 19 }, lineHeight: 1.25, fontWeight: 900, color: tokens.color.text.primary }}>
            Build a bilingual customer portal with signup, payments, admin roles and mobile.
          </Typography>
        </Box>

        <Stack direction="row" spacing={0.8} sx={{ mt: 1.6, mb: 2 }}>
          {tabs.map((tab, index) => (
            <Box
              key={tab}
              sx={{
                px: 1.15,
                py: 0.55,
                borderRadius: 999,
                bgcolor: index === 1 ? tokens.color.accent.violet : "transparent",
                color: index === 1 ? tokens.color.text.primary : tokens.color.text.secondary,
                fontSize: 11,
                fontWeight: 900,
              }}
            >
              {tab}
            </Box>
          ))}
        </Stack>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "repeat(2, minmax(0, 1fr))",
            gap: 1.2,
          }}
        >
          {workCards.map((card) => (
            <Box
              key={card.label}
              sx={{
                p: 1.6,
                minHeight: 132,
                borderRadius: 1.4,
                bgcolor: `${tokens.color.bg.surfaceRaised}dc`,
                border: `1px solid ${tokens.color.border.subtle}`,
              }}
            >
              <Stack direction="row" spacing={0.8} alignItems="center" sx={{ color: tokens.color.accent.violet }}>
                <Box sx={{ display: "grid", "& svg": { fontSize: 15 } }}>{card.icon}</Box>
                <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 10.5, fontWeight: 900 }}>
                  {card.label}
                </Typography>
              </Stack>
              <Typography sx={{ mt: 0.8, fontSize: 13.5, fontWeight: 900, color: tokens.color.text.primary }}>
                {card.title}
              </Typography>
              <Typography sx={{ mt: 0.5, fontSize: 11.5, lineHeight: 1.45, color: tokens.color.text.secondary }}>
                {card.body}
              </Typography>
            </Box>
          ))}

          <Box
            sx={{
              p: 1.6,
              minHeight: 132,
              borderRadius: 1.4,
              border: `1px solid ${tokens.color.border.subtle}`,
              bgcolor: `${tokens.color.bg.inset}dd`,
            }}
          >
            <Stack spacing={1}>
              {checklist.map((item) => (
                <Stack key={item} direction="row" spacing={0.7} alignItems="center">
                  <CheckCircleRounded sx={{ fontSize: 15, color: tokens.color.brand.mint }} />
                  <Typography sx={{ fontSize: 12.2, fontWeight: 900, color: tokens.color.text.primary }}>
                    {item}
                  </Typography>
                </Stack>
              ))}
              <Box
                sx={{
                  mt: 1.1,
                  height: 34,
                  borderRadius: 1,
                  display: "grid",
                  placeItems: "center",
                  border: `1px solid ${tokens.color.accent.violet}30`,
                  bgcolor: `${tokens.color.accent.violet}13`,
                  color: tokens.color.accent.violet,
                }}
              >
                <RocketLaunchRounded sx={{ fontSize: 18 }} />
              </Box>
            </Stack>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
