// MechanicCard — names a real Ironflyer mechanic (gate, ledger entry,
// runtime driver) and gives a 1–2 line description. The mechanic name
// is rendered in mono so it reads as a concrete code-level noun, not
// marketing copy.

import { Box, Stack, Typography } from "@mui/material";
import type { ReactNode } from "react";
import { tokens } from "../../theme";

export interface MechanicCardProps {
  name: string;
  description: string;
  icon?: ReactNode;
  accent?: "violet" | "coral" | "mint";
}

export function MechanicCard({
  name,
  description,
  icon,
  accent = "violet",
}: MechanicCardProps) {
  const accentColor =
    accent === "coral"
      ? tokens.color.accent.coral
      : accent === "mint"
        ? tokens.color.accent.success
        : tokens.color.accent.violet;

  return (
    <Box
      sx={{
        p: { xs: 2.4, md: 2.8 },
        borderRadius: `${tokens.radius.md}px`,
        border: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: `${tokens.color.bg.surfaceRaised}cc`,
        transition: `border-color ${tokens.motion.base} ${tokens.motion.curve}, background ${tokens.motion.base} ${tokens.motion.curve}`,
        height: "100%",
        display: "flex",
        flexDirection: "column",
        gap: 1.2,
        "&:hover": {
          borderColor: `${accentColor}66`,
          bgcolor: tokens.color.bg.surfaceHover,
        },
      }}
    >
      <Stack direction="row" alignItems="center" spacing={1.2}>
        {icon && (
          <Box
            sx={{
              display: "inline-grid",
              placeItems: "center",
              width: 32,
              height: 32,
              borderRadius: `${tokens.radius.sm}px`,
              bgcolor: `${accentColor}1f`,
              color: accentColor,
              "& svg": { fontSize: 18 },
            }}
          >
            {icon}
          </Box>
        )}
        <Typography
          sx={{
            fontFamily: tokens.font.mono,
            fontSize: 13.5,
            fontWeight: 700,
            color: tokens.color.text.primary,
            letterSpacing: 0.2,
          }}
        >
          {name}
        </Typography>
      </Stack>
      <Typography
        sx={{
          color: tokens.color.text.secondary,
          fontSize: 13.5,
          lineHeight: 1.55,
        }}
      >
        {description}
      </Typography>
    </Box>
  );
}
