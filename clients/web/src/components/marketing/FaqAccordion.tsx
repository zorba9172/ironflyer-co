"use client";

// FaqAccordion — vertical stack of MUI Accordion items. Marked client
// because MUI Accordion owns expansion state. Visual styling stays
// inside the token system; no raw hex/rgba.

import { ExpandMoreRounded } from "@mui/icons-material";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Typography,
} from "@mui/material";
import { tokens } from "../../theme";

export interface FaqItem {
  question: string;
  answer: string;
}

export interface FaqAccordionProps {
  items: FaqItem[];
}

export function FaqAccordion({ items }: FaqAccordionProps) {
  return (
    <Box
      sx={{
        borderRadius: `${tokens.radius.md}px`,
        border: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: `${tokens.color.bg.surface}cc`,
        overflow: "hidden",
      }}
    >
      {items.map((item, idx) => (
        <Accordion
          key={item.question}
          disableGutters
          square
          elevation={0}
          sx={{
            bgcolor: "transparent",
            borderBottom:
              idx < items.length - 1
                ? `1px solid ${tokens.color.border.subtle}`
                : "none",
            "&:before": { display: "none" },
          }}
        >
          <AccordionSummary
            expandIcon={
              <ExpandMoreRounded
                sx={{ color: tokens.color.accent.violet }}
              />
            }
            sx={{
              px: { xs: 2.4, md: 3 },
              py: 0.5,
              "& .MuiAccordionSummary-content": { my: 1.8 },
            }}
          >
            <Typography
              sx={{
                fontSize: { xs: 15, md: 16 },
                fontWeight: 700,
                color: tokens.color.text.primary,
                letterSpacing: -0.1,
              }}
            >
              {item.question}
            </Typography>
          </AccordionSummary>
          <AccordionDetails sx={{ px: { xs: 2.4, md: 3 }, pt: 0, pb: 2.8 }}>
            <Typography
              sx={{
                color: tokens.color.text.secondary,
                fontSize: 14.5,
                lineHeight: 1.65,
                maxWidth: 820,
              }}
            >
              {item.answer}
            </Typography>
          </AccordionDetails>
        </Accordion>
      ))}
    </Box>
  );
}
