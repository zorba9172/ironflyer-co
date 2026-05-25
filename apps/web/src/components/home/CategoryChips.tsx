"use client";

// CategoryChips — eight one-click seed prompts under the hero composer.
// Clicking a chip writes a concrete starting line into the prompt
// (lifted to the page) and focuses the textarea so the user can
// immediately refine. The "More" chip opens a dialog with ten extra
// seeds so we keep the row short on mobile.

import {
  ApiOutlined,
  AppRegistrationOutlined,
  CalendarMonthOutlined,
  CloseRounded,
  EditNoteOutlined,
  EventAvailableOutlined,
  HubOutlined,
  PhoneIphoneOutlined,
  PointOfSaleOutlined,
  StackedLineChartOutlined,
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
    label: "Tasks & Workflows",
    seed: "An internal task tracker with assignments, due dates, and a weekly digest email.",
    Icon: AppRegistrationOutlined,
  },
  {
    label: "CRM & Sales",
    seed: "A CRM with contacts, deals, and a kanban pipeline; notes and follow-up reminders.",
    Icon: HubOutlined,
  },
  {
    label: "Content & Sites",
    seed: "A blog with admin, Markdown posts, tag search, and an RSS feed.",
    Icon: EditNoteOutlined,
  },
  {
    label: "Finance",
    seed: "An expense tracker for a small team with receipts, categories, and a monthly report.",
    Icon: PointOfSaleOutlined,
  },
  {
    label: "Booking",
    seed: "An appointment booking site for my service business with deposits and calendar sync.",
    Icon: EventAvailableOutlined,
  },
  {
    label: "Mobile",
    seed: "A mobile app to log daily meals with photo capture and a weekly summary (Expo).",
    Icon: PhoneIphoneOutlined,
  },
  {
    label: "API",
    seed: "A Go HTTP API for managing inventory with auth, audit log, and a CSV export.",
    Icon: ApiOutlined,
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
  onPick: (seed: string) => void;
}

export function CategoryChips({ onPick }: CategoryChipsProps) {
  const [moreOpen, setMoreOpen] = useState(false);

  return (
    <>
      <Stack
        direction="row"
        useFlexGap
        flexWrap="wrap"
        spacing={1}
        sx={{ width: "100%", rowGap: 1, justifyContent: "center" }}
      >
        {CHIPS.map(({ label, seed, Icon }) => (
          <Button
            key={label}
            size="small"
            onClick={() => onPick(seed)}
            startIcon={<Icon sx={{ fontSize: 16 }} />}
            sx={chipSx()}
          >
            {label}
          </Button>
        ))}
        <Button
          size="small"
          onClick={() => setMoreOpen(true)}
          startIcon={<StackedLineChartOutlined sx={{ fontSize: 16 }} />}
          sx={chipSx()}
        >
          More
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
              bgcolor: tokens.color.bg.surface,
              border: `1px solid ${tokens.color.border.subtle}`,
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
              borderBottom: `1px solid ${tokens.color.border.subtle}`,
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
              sx={{ color: tokens.color.text.secondary }}
            >
              <CloseRounded fontSize="small" />
            </IconButton>
          </Stack>
          <Stack divider={<Box sx={{ height: 1, bgcolor: tokens.color.border.subtle }} />}>
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
                  "&:hover": { bgcolor: tokens.color.bg.surfaceHover },
                }}
              >
                <Typography sx={{ fontSize: 13.5, fontWeight: 700 }}>
                  {it.title}
                </Typography>
                <Typography
                  sx={{
                    fontSize: 12.5,
                    color: tokens.color.text.secondary,
                    mt: 0.25,
                  }}
                >
                  {it.seed}
                </Typography>
              </Box>
            ))}
          </Stack>
          <Box
            sx={{
              px: 2.5,
              py: 1,
              borderTop: `1px solid ${tokens.color.border.subtle}`,
              bgcolor: tokens.color.bg.inset,
            }}
          >
            <Typography
              sx={{ fontSize: 11, color: tokens.color.text.muted }}
            >
              These are seed prompts — refine the wording before launch.
            </Typography>
          </Box>
        </DialogContent>
      </Dialog>
    </>
  );
}

function chipSx() {
  return {
    color: tokens.color.text.secondary,
    bgcolor: tokens.color.bg.surface,
    border: `1px solid ${tokens.color.border.subtle}`,
    borderRadius: 999,
    px: 1.5,
    py: 0.5,
    minHeight: 32,
    fontWeight: 600,
    fontSize: 12.5,
    letterSpacing: 0.1,
    "&:hover": {
      bgcolor: tokens.color.bg.surfaceHover,
      borderColor: tokens.color.accent.violet,
      color: tokens.color.text.primary,
    },
  };
}
