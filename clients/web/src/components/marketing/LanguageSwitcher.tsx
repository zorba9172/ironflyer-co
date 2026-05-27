"use client";

import { ExpandMoreRounded } from "@mui/icons-material";
import { Button, Menu, MenuItem, Stack, Typography } from "@mui/material";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { tokens } from "../../theme";
import { SUPPORTED_LOCALES, type Locale } from "../../lib/i18n/content";
import { useI18n } from "../../lib/i18n/useI18n";

export function LanguageSwitcher() {
  const router = useRouter();
  const { locale, setLocale } = useI18n();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const active =
    SUPPORTED_LOCALES.find((l) => l.code === locale) ?? SUPPORTED_LOCALES[0];

  const pick = (next: Locale) => {
    setAnchor(null);
    setLocale(next);
    router.refresh();
  };

  return (
    <>
      <Button
        size="small"
        onClick={(e) => setAnchor(e.currentTarget)}
        sx={{
          minWidth: 0,
          px: { xs: 0.9, sm: 1.05 },
          color: tokens.color.text.secondary,
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: `${tokens.color.bg.surfaceRaised}80`,
          fontFamily: tokens.font.mono,
          fontSize: 12,
          borderRadius: 999,
          "&:hover": {
            color: tokens.color.text.primary,
            bgcolor: tokens.color.bg.surfaceHover,
          },
        }}
        aria-haspopup="menu"
        aria-expanded={!!anchor}
      >
        <Typography component="span" sx={{ fontSize: 16, lineHeight: 1 }}>
          {active.flag}
        </Typography>
        <Typography
          component="span"
          sx={{
            display: { xs: "none", sm: "inline" },
            fontFamily: tokens.font.mono,
            fontSize: 12,
            ml: 0.65,
          }}
        >
          {active.short}
        </Typography>
        <ExpandMoreRounded
          sx={{ display: { xs: "none", sm: "block" }, fontSize: 15, ml: 0.35 }}
        />
      </Button>
      <Menu
        anchorEl={anchor}
        open={!!anchor}
        onClose={() => setAnchor(null)}
        slotProps={{
          paper: {
            sx: {
              mt: 1,
              minWidth: 170,
              border: `1px solid ${tokens.color.border.subtle}`,
              bgcolor: tokens.color.bg.surfaceRaised,
            },
          },
        }}
      >
        {SUPPORTED_LOCALES.map((option) => (
          <MenuItem key={option.code} onClick={() => pick(option.code)}>
            <Stack direction="row" spacing={1} alignItems="center">
              <Typography sx={{ fontSize: 17, lineHeight: 1 }}>
                {option.flag}
              </Typography>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 12,
                  color: tokens.color.accent.violet,
                }}
              >
                {option.short}
              </Typography>
              <Typography
                sx={{
                  fontSize: 13,
                  fontWeight: option.code === locale ? 800 : 600,
                }}
              >
                {option.label}
              </Typography>
            </Stack>
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}
