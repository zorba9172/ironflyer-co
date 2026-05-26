import {
  AccountTreeRounded,
  CheckCircleRounded,
  CodeRounded,
  RocketLaunchRounded,
  SecurityRounded,
} from "@mui/icons-material";
import { Box, Stack, Typography } from "@mui/material";
import { tokens } from "../../theme";

const events = [
  ["00:01", "intent.parsed", "SaaS dashboard + mobile companion"],
  ["00:08", "workspace.ready", "Next.js, API, auth, database"],
  ["00:16", "patch.proposed", "+37 files, reviewable diff"],
  ["00:24", "gates.green", "typecheck, build, security, E2E"],
  ["00:30", "preview.live", "project opened in Studio"],
] as const;

export function ProductTheater() {
  return (
    <Box
      sx={{
        position: "relative",
        minHeight: { xs: 430, md: 520 },
        borderRadius: 2,
        overflow: "hidden",
        border: `1px solid ${tokens.color.border.strong}`,
        bgcolor: `${tokens.color.bg.inset}e6`,
        boxShadow: `0 26px 90px ${tokens.color.accent.purple}33`,
      }}
    >
      <Box
        component="video"
        src="/market/ai-replies-loop.mp4"
        autoPlay
        muted
        loop
        playsInline
        sx={{
          position: "absolute",
          inset: 0,
          width: "100%",
          height: "100%",
          objectFit: "cover",
          opacity: 0.24,
          filter: "saturate(1.4) contrast(1.1)",
        }}
      />
      <Box
        sx={{
          position: "absolute",
          inset: 0,
          background: `linear-gradient(180deg, ${tokens.color.bg.inset}52, ${tokens.color.bg.inset}f5 72%)`,
        }}
      />
      <Box sx={{ position: "relative", p: { xs: 2, md: 2.6 }, height: "100%" }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between">
          <Stack direction="row" spacing={0.8} alignItems="center">
            <Box sx={{ width: 9, height: 9, borderRadius: "50%", bgcolor: tokens.color.accent.red }} />
            <Box sx={{ width: 9, height: 9, borderRadius: "50%", bgcolor: tokens.color.accent.yellow }} />
            <Box sx={{ width: 9, height: 9, borderRadius: "50%", bgcolor: tokens.color.brand.mint }} />
          </Stack>
          <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 11, color: tokens.color.text.muted }}>
            IRONFLYER / LIVE EXECUTION
          </Typography>
        </Stack>

        <Box
          sx={{
            mt: 2.4,
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "0.95fr 1.05fr" },
            gap: 1.4,
          }}
        >
          <Stack spacing={1.2}>
            {[
              { icon: <CodeRounded />, label: "Codebase", value: "Generated + editable" },
              { icon: <SecurityRounded />, label: "Security", value: "Gates blocking" },
              { icon: <AccountTreeRounded />, label: "System", value: "Frontend / API / DB" },
              { icon: <RocketLaunchRounded />, label: "Deploy", value: "Preview ready" },
            ].map((row) => (
              <Stack
                key={row.label}
                direction="row"
                spacing={1}
                alignItems="center"
                sx={{
                  p: 1.2,
                  borderRadius: 1,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  bgcolor: `${tokens.color.bg.surfaceRaised}bd`,
                }}
              >
                <Box
                  sx={{
                    width: 32,
                    height: 32,
                    borderRadius: 1,
                    display: "grid",
                    placeItems: "center",
                    color: tokens.color.accent.violet,
                    bgcolor: `${tokens.color.accent.violet}18`,
                    "& svg": { fontSize: 18 },
                  }}
                >
                  {row.icon}
                </Box>
                <Box sx={{ minWidth: 0 }}>
                  <Typography sx={{ fontSize: 12, color: tokens.color.text.muted, fontFamily: tokens.font.mono }}>
                    {row.label}
                  </Typography>
                  <Typography sx={{ fontSize: 13.5, fontWeight: 800, color: tokens.color.text.primary }}>
                    {row.value}
                  </Typography>
                </Box>
              </Stack>
            ))}
          </Stack>

          <Box
            sx={{
              borderRadius: 1,
              border: `1px solid ${tokens.color.border.subtle}`,
              bgcolor: `${tokens.color.bg.base}bf`,
              overflow: "hidden",
            }}
          >
            <Box
              sx={{
                px: 1.4,
                py: 1,
                borderBottom: `1px solid ${tokens.color.border.subtle}`,
                display: "flex",
                justifyContent: "space-between",
              }}
            >
              <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 11, color: tokens.color.text.muted }}>
                execution.stream
              </Typography>
              <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 11, color: tokens.color.brand.mint }}>
                running
              </Typography>
            </Box>
            <Stack spacing={0} sx={{ p: 1.1 }}>
              {events.map(([time, event, detail], index) => (
                <Stack
                  key={event}
                  direction="row"
                  spacing={1}
                  sx={{
                    py: 0.82,
                    opacity: 0,
                    animation: `ironflyerEventIn 9s ${index * 0.45}s ease-in-out infinite`,
                    "@keyframes ironflyerEventIn": {
                      "0%, 7%": { opacity: 0, transform: "translateY(6px)" },
                      "12%, 82%": { opacity: 1, transform: "translateY(0)" },
                      "92%, 100%": { opacity: 0.45, transform: "translateY(0)" },
                    },
                  }}
                >
                  <Typography sx={{ width: 42, fontFamily: tokens.font.mono, fontSize: 11, color: tokens.color.text.muted }}>
                    {time}
                  </Typography>
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Stack direction="row" spacing={0.6} alignItems="center">
                      <CheckCircleRounded sx={{ fontSize: 14, color: tokens.color.brand.mint }} />
                      <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 12, color: tokens.color.text.primary }}>
                        {event}
                      </Typography>
                    </Stack>
                    <Typography sx={{ mt: 0.25, fontSize: 11.5, color: tokens.color.text.secondary }}>
                      {detail}
                    </Typography>
                  </Box>
                </Stack>
              ))}
            </Stack>
          </Box>
        </Box>

        <Box
          sx={{
            mt: 1.4,
            p: 1.4,
            borderRadius: 1,
            border: `1px solid ${tokens.color.accent.violet}3d`,
            bgcolor: `${tokens.color.accent.violet}10`,
          }}
        >
          <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 12, color: tokens.color.text.secondary }}>
            prompt: "Build a multilingual booking SaaS with Stripe, admin dashboard and mobile preview"
          </Typography>
        </Box>
      </Box>
    </Box>
  );
}
