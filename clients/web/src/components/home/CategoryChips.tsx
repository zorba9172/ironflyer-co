"use client";

import {
  CloseRounded,
  HubOutlined,
  Inventory2Outlined,
  PhoneIphoneOutlined,
  SpaceDashboardOutlined,
  StorefrontOutlined,
  ViewKanbanOutlined,
} from "@mui/icons-material";
import {
  Box,
  Button,
  Dialog,
  DialogContent,
  IconButton,
  Stack,
  Typography,
} from "@mui/material";
import type { SvgIconComponent } from "@mui/icons-material";
import { useState } from "react";
import { tokens } from "../../theme";

interface CategoryChip {
  label: string;
  seed: string;
  Icon: SvgIconComponent;
}

const CHIPS: CategoryChip[] = [
  {
    label: "SaaS dashboard",
    seed: "A SaaS dashboard with users, billing, roles, and product analytics.",
    Icon: SpaceDashboardOutlined,
  },
  {
    label: "Marketplace",
    seed: "A marketplace with listings, search, checkout, seller profiles, and admin moderation.",
    Icon: StorefrontOutlined,
  },
  {
    label: "Internal tool",
    seed: "An internal tool for approvals with workflows, comments, roles, and reporting.",
    Icon: HubOutlined,
  },
  {
    label: "Admin panel",
    seed: "An admin panel with tables, charts, filters, permissions, and audit logs.",
    Icon: ViewKanbanOutlined,
  },
  {
    label: "Mobile app",
    seed: "A mobile app with onboarding, profile, push notifications, and a synced dashboard.",
    Icon: PhoneIphoneOutlined,
  },
];

const MORE_IDEAS: { title: string; seed: string }[] = [
  {
    title: "Customer support inbox",
    seed: "A shared inbox where teammates can claim tickets, tag, and reply via email.",
  },
  {
    title: "Property listing site",
    seed: "A small real-estate site with listings, photo galleries, and a contact-the-agent form.",
  },
  {
    title: "Online course platform",
    seed: "A course platform with modules, video lessons, progress tracking, and a certificate at the end.",
  },
  {
    title: "Team standup logger",
    seed: "A daily standup logger that DMs each teammate at 9am and posts the summary to Slack.",
  },
  {
    title: "Newsletter publisher",
    seed: "A newsletter publisher with a public archive, subscriber list, and Stripe-backed paid tier.",
  },
  {
    title: "Subscription billing portal",
    seed: "A Stripe-backed customer portal where users see invoices, swap plans, and cancel.",
  },
  {
    title: "Restaurant menu + ordering",
    seed: "A restaurant menu site with online ordering, prep tickets, and a daily summary.",
  },
  {
    title: "Habit tracker (PWA)",
    seed: "A habit tracker PWA with streaks, reminders, and a weekly review.",
  },
  {
    title: "Volunteer scheduler",
    seed: "A volunteer scheduler with shift signup, swap requests, and a manager dashboard.",
  },
  {
    title: "Discord moderation bot",
    seed: "A Discord moderation bot with audit logs, slow-mode, and a warning ledger.",
  },
];

export interface CategoryChipsProps {
  timing?: "dark" | "light";
  onPick: (seed: string) => void;
}

export function CategoryChips({ timing = "dark", onPick }: CategoryChipsProps) {
  const [moreOpen, setMoreOpen] = useState(false);
  const light = timing === "light";

  return (
    <>
      <Stack
        direction="row"
        spacing={0.8}
        sx={{
          width: "100%",
          justifyContent: "center",
          overflowX: "auto",
          overflowY: "hidden",
          scrollbarWidth: "none",
          "&::-webkit-scrollbar": { display: "none" },
        }}
      >
        {CHIPS.map(({ label, seed, Icon }) => (
          <Button
            key={label}
            size="small"
            onClick={() => onPick(seed)}
            startIcon={<Icon sx={{ fontSize: 16 }} />}
            sx={chipSx(light)}
          >
            {label}
          </Button>
        ))}
        <Button
          size="small"
          onClick={() => setMoreOpen(true)}
          startIcon={<Inventory2Outlined sx={{ fontSize: 16 }} />}
          sx={chipSx(light)}
        >
          More templates
        </Button>
      </Stack>

      <Dialog
        open={moreOpen}
        onClose={() => setMoreOpen(false)}
        maxWidth="sm"
        fullWidth
        slotProps={{
          paper: {
            sx: {
              bgcolor: light ? "#fff" : tokens.color.bg.surface,
              border: `1px solid ${light ? "rgba(127,77,255,0.16)" : tokens.color.border.subtle}`,
              backgroundImage: "none",
            },
          },
        }}
      >
        <DialogContent sx={{ p: 0 }}>
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            sx={{
              px: 2.5,
              py: 1.5,
              borderBottom: `1px solid ${light ? "rgba(127,77,255,0.16)" : tokens.color.border.subtle}`,
            }}
          >
            <Box>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  letterSpacing: 1.4,
                  color: tokens.color.accent.violet,
                  textTransform: "uppercase",
                }}
              >
                Seed ideas
              </Typography>
              <Typography sx={{ fontSize: 15, fontWeight: 700, mt: 0.25 }}>
                Pick a starting line
              </Typography>
            </Box>
            <IconButton
              size="small"
              onClick={() => setMoreOpen(false)}
              sx={{ color: light ? "#677092" : tokens.color.text.secondary }}
            >
              <CloseRounded fontSize="small" />
            </IconButton>
          </Stack>
          <Stack divider={<Box sx={{ height: 1, bgcolor: light ? "rgba(127,77,255,0.12)" : tokens.color.border.subtle }} />}>
            {MORE_IDEAS.map((it) => (
              <Box
                key={it.title}
                role="button"
                tabIndex={0}
                onClick={() => {
                  onPick(it.seed);
                  setMoreOpen(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onPick(it.seed);
                    setMoreOpen(false);
                  }
                }}
                sx={{
                  px: 2.5,
                  py: 1.5,
                  cursor: "pointer",
                  "&:hover": { bgcolor: light ? "rgba(143,77,255,0.08)" : tokens.color.bg.surfaceHover },
                }}
              >
                <Typography sx={{ fontSize: 13.5, fontWeight: 700 }}>
                  {it.title}
                </Typography>
                <Typography
                  sx={{
                    fontSize: 12.5,
                    color: light ? "#677092" : tokens.color.text.secondary,
                    mt: 0.25,
                  }}
                >
                  {it.seed}
                </Typography>
              </Box>
            ))}
          </Stack>
        </DialogContent>
      </Dialog>
    </>
  );
}

function chipSx(light: boolean) {
  return {
    color: light ? "#677092" : tokens.color.text.secondary,
    bgcolor: light ? "rgba(255,255,255,0.72)" : tokens.color.bg.surface,
    border: `1px solid ${light ? "rgba(127,77,255,0.16)" : tokens.color.border.subtle}`,
    borderRadius: 999,
    px: 1.65,
    py: 0.58,
    minHeight: 36,
    flexShrink: 0,
    fontWeight: 800,
    fontSize: 12.5,
    letterSpacing: 0.1,
    "&:hover": {
      bgcolor: light ? "rgba(143,77,255,0.08)" : tokens.color.bg.surfaceHover,
      borderColor: tokens.color.accent.violet,
      color: light ? "#171b44" : tokens.color.text.primary,
    },
  };
}
