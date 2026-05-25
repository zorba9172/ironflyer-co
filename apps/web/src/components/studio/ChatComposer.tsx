"use client";

// ChatComposer — autosizing textarea + send button. Enter submits,
// Shift+Enter inserts a newline. The send button is lime when armed
// and faded when disabled. Caller wires the actual mutation (refineIdea
// or a raw fallback) — this component is presentation-only.

import {
  AttachFileRounded,
  SendRounded,
  StopCircleOutlined,
} from "@mui/icons-material";
import {
  Box,
  CircularProgress,
  IconButton,
  Stack,
  Tooltip,
} from "@mui/material";
import { useCallback, useEffect, useRef, useState } from "react";
import { tokens } from "../../theme";

export interface ChatComposerProps {
  onSend: (message: string) => void | Promise<void>;
  onStop?: () => void;
  disabled?: boolean;
  pending?: boolean;
  placeholder?: string;
  modelLabel?: string;
}

const MIN_HEIGHT = 44;
const MAX_HEIGHT = 200;

export function ChatComposer({
  onSend,
  onStop,
  disabled,
  pending,
  placeholder = "Tell Ironflyer what to change…",
  modelLabel = "Claude Sonnet 4.6",
}: ChatComposerProps) {
  const [value, setValue] = useState("");
  const ref = useRef<HTMLTextAreaElement | null>(null);

  const autoResize = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    const next = Math.min(MAX_HEIGHT, Math.max(MIN_HEIGHT, el.scrollHeight));
    el.style.height = `${next}px`;
  }, []);

  useEffect(() => {
    autoResize();
  }, [value, autoResize]);

  const armed = !disabled && !pending && value.trim().length > 0;

  const submit = useCallback(async () => {
    const text = value.trim();
    if (!text || disabled || pending) return;
    setValue("");
    await onSend(text);
  }, [value, disabled, pending, onSend]);

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      void submit();
    }
  };

  return (
    <Box
      sx={{
        borderTop: `1px solid ${tokens.color.border.subtle}`,
        bgcolor: tokens.color.bg.surface,
        px: 1.5,
        py: 1.25,
      }}
    >
      <Box
        sx={{
          border: `1px solid ${
            armed ? tokens.color.accent.violet + "55" : tokens.color.border.subtle
          }`,
          bgcolor: tokens.color.bg.inset,
          borderRadius: 1.5,
          transition: "border-color 160ms ease",
          "&:focus-within": {
            borderColor: tokens.color.accent.violet,
          },
        }}
      >
        <Box
          component="textarea"
          ref={ref}
          value={value}
          onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
            setValue(e.target.value)
          }
          onKeyDown={onKeyDown}
          placeholder={placeholder}
          rows={1}
          aria-label="Send a message to Ironflyer"
          disabled={disabled}
          sx={{
            background: "transparent",
            border: "none",
            color: tokens.color.text.primary,
            display: "block",
            fontFamily: tokens.font.family,
            fontSize: 14,
            lineHeight: 1.5,
            outline: "none",
            px: 1.5,
            py: 1.25,
            resize: "none",
            width: "100%",
            "&::placeholder": { color: tokens.color.text.muted },
            "&:disabled": { opacity: 0.5 },
          }}
        />
        <Stack
          direction="row"
          spacing={1}
          sx={{
            alignItems: "center",
            borderTop: `1px solid ${tokens.color.border.subtle}`,
            px: 1,
            py: 0.5,
          }}
        >
          <Tooltip title="Attach (coming soon)" arrow>
            <span>
              <IconButton
                size="small"
                disabled
                sx={{
                  color: tokens.color.text.muted,
                  "&.Mui-disabled": { color: tokens.color.text.muted },
                }}
                aria-label="Attach file"
              >
                <AttachFileRounded sx={{ fontSize: 16 }} />
              </IconButton>
            </span>
          </Tooltip>
          <Box
            sx={{
              color: tokens.color.text.muted,
              fontFamily: tokens.font.mono,
              fontSize: 10.5,
              letterSpacing: 0.4,
              textTransform: "uppercase",
            }}
          >
            {modelLabel}
          </Box>
          <Box sx={{ flex: 1 }} />
          {pending && onStop ? (
            <Tooltip title="Stop generation" arrow>
              <IconButton
                size="small"
                onClick={onStop}
                sx={{ color: tokens.color.accent.danger }}
                aria-label="Stop"
              >
                <StopCircleOutlined sx={{ fontSize: 18 }} />
              </IconButton>
            </Tooltip>
          ) : null}
          <Tooltip title={armed ? "Send (Enter)" : "Write something to send"} arrow>
            <span>
              <IconButton
                size="small"
                onClick={() => void submit()}
                disabled={!armed}
                aria-label="Send"
                sx={{
                  background: armed
                    ? `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`
                    : tokens.color.bg.surfaceRaised,
                  color: armed ? tokens.color.text.primary : tokens.color.text.muted,
                  borderRadius: 1,
                  width: 32,
                  height: 32,
                  "&:hover": {
                    background: armed
                      ? `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.purple})`
                      : tokens.color.bg.surfaceRaised,
                    filter: armed ? "brightness(1.06)" : "none",
                  },
                  "&.Mui-disabled": {
                    bgcolor: tokens.color.bg.surfaceRaised,
                    color: tokens.color.text.muted,
                  },
                }}
              >
                {pending ? (
                  <CircularProgress
                    size={14}
                    thickness={6}
                    sx={{ color: tokens.color.text.muted }}
                  />
                ) : (
                  <SendRounded sx={{ fontSize: 16 }} />
                )}
              </IconButton>
            </span>
          </Tooltip>
        </Stack>
      </Box>
    </Box>
  );
}
