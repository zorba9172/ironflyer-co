"use client";

import { LanguageRounded } from "@mui/icons-material";
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
  const active = SUPPORTED_LOCALES.find((l) => l.code === locale) ?? SUPPORTED_LOCALES[0];

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
        startIcon={<LanguageRounded sx={{ fontSize: 17 }} />}
        sx={{
          minWidth: 0,
          px: { xs: 0.9, sm: 1.15 },
          color: tokens.color.text.secondary,
          border: `1px solid ${tokens.color.border.subtle}`,
          bgcolor: `${tokens.color.bg.surfaceRaised}80`,
          fontFamily: tokens.font.mono,
          fontSize: 12,
          "&:hover": {
            color: tokens.color.text.primary,
            bgcolor: tokens.color.bg.surfaceHover,
          },
          "& .MuiButton-startIcon": {
            mr: { xs: 0, sm: 0.6 },
          },
        }}
        aria-haspopup="menu"
        aria-expanded={!!anchor}
      >
        <Typography
          component="span"
          sx={{ display: { xs: "none", sm: "inline" }, fontFamily: tokens.font.mono, fontSize: 12 }}
        >
          {active.short}
        </Typography>
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
              <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 12, color: tokens.color.accent.violet }}>
                {option.short}
              </Typography>
              <Typography sx={{ fontSize: 13, fontWeight: option.code === locale ? 800 : 600 }}>
                {option.label}
              </Typography>
            </Stack>
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}
