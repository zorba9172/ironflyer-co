"use client";

import { Box, Stack, Typography } from "@mui/material";
import { tokens } from "../../theme";

export interface CodeTextViewProps {
  value: string;
  language?: string;
  path?: string;
  wordWrap?: boolean;
}

export function CodeTextView({
  value,
  language,
  path,
  wordWrap = false,
}: CodeTextViewProps) {
  const lines = value.split("\n");

  return (
    <Stack sx={{ bgcolor: "#060712", height: "100%", minHeight: 0 }}>
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        sx={{
          borderBottom: `1px solid ${tokens.color.border.subtle}`,
          color: tokens.color.text.muted,
          flexShrink: 0,
          fontFamily: tokens.font.mono,
          fontSize: 10.5,
          minHeight: 30,
          overflow: "hidden",
          px: 1.2,
          textTransform: "uppercase",
        }}
      >
        <Box sx={{ color: tokens.color.text.secondary, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {path || "buffer"}
        </Box>
        <Box sx={{ flex: 1 }} />
        <Box>{language || "text"}</Box>
      </Stack>
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          overflow: "auto",
          p: 1.4,
        }}
      >
        <Box component="pre" sx={{ m: 0, minWidth: wordWrap ? 0 : 720 }}>
          {lines.map((line, index) => (
            <Box
              key={`${index}-${line.slice(0, 24)}`}
              sx={{
                display: "grid",
                gridTemplateColumns: "44px minmax(0, 1fr)",
                fontFamily: tokens.font.mono,
                fontSize: 13,
                lineHeight: 1.72,
              }}
            >
              <Typography
                component="span"
                sx={{
                  color: tokens.color.text.muted,
                  fontFamily: tokens.font.mono,
                  fontSize: 12,
                  pr: 1.3,
                  textAlign: "right",
                  userSelect: "none",
                }}
              >
                {index + 1}
              </Typography>
              <Typography
                component="span"
                sx={{
                  color: tokenColor(line),
                  fontFamily: tokens.font.mono,
                  fontSize: 13,
                  overflowWrap: wordWrap ? "anywhere" : "normal",
                  whiteSpace: wordWrap ? "pre-wrap" : "pre",
                }}
              >
                {line || " "}
              </Typography>
            </Box>
          ))}
        </Box>
      </Box>
    </Stack>
  );
}

function tokenColor(line: string): string {
  const trimmed = line.trim();
  if (trimmed.startsWith("//") || trimmed.startsWith("#")) return "#7380a5";
  if (/^(import|export|type|interface|const|let|function|return)\b/.test(trimmed)) {
    return "#bd93f9";
  }
  if (/["'`]/.test(line)) return "#8be9fd";
  if (/[{}()[\]]/.test(line)) return "#f8f8f2";
  return "#d7dcff";
}
