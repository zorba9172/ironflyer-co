"use client";

// CodeBlock — monospace preformatted view for diff hashes, ledger
// hashes, error payloads, JSON metadata. Optional copy button + label.
// No syntax highlighting yet — keep the surface honest until a
// highlighter belongs in the bundle.

import { ContentCopyRounded, DoneRounded } from "@mui/icons-material";
import { Box, IconButton, Stack, Tooltip, Typography, type SxProps, type Theme } from "@mui/material";
import { useState } from "react";
import { tokens } from "../../theme";

export interface CodeBlockProps {
  code: string;
  language?: string;
  // Optional badge above the block ("response.json", "diff hash", …).
  label?: string;
  // Render inline (no card) — useful inside table cells.
  inline?: boolean;
  // When true and the content is single-line, render compact.
  compact?: boolean;
  // Hide the copy button (e.g. for short hashes already shown elsewhere).
  noCopy?: boolean;
  sx?: SxProps<Theme>;
}

export function CodeBlock({
  code,
  language,
  label,
  inline,
  compact,
  noCopy,
  sx,
}: CodeBlockProps) {
  const [copied, setCopied] = useState(false);

  async function copy() {
    if (typeof navigator === "undefined" || !navigator.clipboard) return;
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      // clipboard write can fail on http origins / restricted contexts —
      // silently ignore, the user can still select + copy manually.
    }
  }

  if (inline) {
    return (
      <Box
        component="code"
        sx={{
          display: "inline-block",
          fontFamily: tokens.font.mono,
          fontSize: 12,
          color: tokens.color.text.primary,
          bgcolor: tokens.color.bg.inset,
          border: `1px solid ${tokens.color.border.subtle}`,
          borderRadius: 0.75,
          px: 0.75,
          py: 0.25,
          ...sx,
        }}
      >
        {code}
      </Box>
    );
  }

  return (
    <Box
      sx={{
        border: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: tokens.color.bg.inset,
        borderRadius: 1,
        overflow: "hidden",
        ...sx,
      }}
    >
      {(label || language || !noCopy) && (
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{
            px: 1.5,
            py: 0.5,
            borderBottom: `1px solid ${tokens.color.border.subtle}`,
            bgcolor: tokens.color.bg.surface,
          }}
        >
          <Stack direction="row" spacing={1} alignItems="center">
            {label && (
              <Typography
                variant="overline"
                sx={{ color: tokens.color.text.muted, fontSize: 10 }}
              >
                {label}
              </Typography>
            )}
            {language && (
              <Typography
                variant="overline"
                sx={{ color: tokens.color.text.secondary, fontSize: 10 }}
              >
                {language}
              </Typography>
            )}
          </Stack>
          {!noCopy && (
            <Tooltip title={copied ? "Copied" : "Copy"} arrow>
              <IconButton size="small" onClick={copy} sx={{ color: tokens.color.text.muted }}>
                {copied ? (
                  <DoneRounded sx={{ fontSize: 16, color: tokens.color.accent.success }} />
                ) : (
                  <ContentCopyRounded sx={{ fontSize: 14 }} />
                )}
              </IconButton>
            </Tooltip>
          )}
        </Stack>
      )}
      <Box
        component="pre"
        sx={{
          m: 0,
          p: compact ? 1 : 1.5,
          overflowX: "auto",
          fontFamily: tokens.font.mono,
          fontSize: compact ? 11.5 : 12.5,
          lineHeight: 1.5,
          color: tokens.color.text.primary,
          whiteSpace: "pre",
        }}
      >
        <code>{code}</code>
      </Box>
    </Box>
  );
}
