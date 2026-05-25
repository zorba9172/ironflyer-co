"use client";

// ChatComposer — autosizing textarea + send button. Enter submits,
// Shift+Enter inserts a newline. The send button is lime when armed
// and faded when disabled. Supports bounded attachments with local
// previews; caller decides how to persist/upload them.

import {
  AttachFileRounded,
  CloseRounded,
  DescriptionRounded,
  ImageRounded,
  SendRounded,
  StopCircleOutlined,
} from "@mui/icons-material";
import {
  Box,
  Button,
  CircularProgress,
  IconButton,
  LinearProgress,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import { useCallback, useEffect, useRef, useState } from "react";
import { tokens } from "../../theme";
import type { StudioAttachment } from "./types";

export interface ChatComposerProps {
  onSend: (message: string, attachments?: StudioAttachment[]) => void | Promise<void>;
  onStop?: () => void;
  disabled?: boolean;
  pending?: boolean;
  placeholder?: string;
  modelLabel?: string;
}

const MIN_HEIGHT = 44;
const MAX_HEIGHT = 200;
const MAX_FILES = 5;
const MAX_FILE_SIZE = 10 * 1024 * 1024;
const MAX_TOTAL_SIZE = 25 * 1024 * 1024;
const BLOCKED_EXTENSIONS = new Set([
  "app",
  "bat",
  "cmd",
  "com",
  "dmg",
  "exe",
  "jar",
  "msi",
  "pkg",
  "scr",
  "sh",
]);
const ALLOWED_MIME_PREFIXES = ["image/", "text/"];
const ALLOWED_MIME_TYPES = new Set([
  "application/json",
  "application/pdf",
  "application/xml",
  "application/x-yaml",
  "application/zip",
  "text/csv",
  "text/markdown",
]);

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function extensionOf(name: string): string {
  const i = name.lastIndexOf(".");
  return i === -1 ? "" : name.slice(i + 1).toLowerCase();
}

function kindFor(file: File): StudioAttachment["kind"] {
  if (file.type.startsWith("image/")) return "image";
  const ext = extensionOf(file.name);
  if (["ts", "tsx", "js", "jsx", "css", "html", "go", "py", "sql"].includes(ext)) return "code";
  if (["csv", "json", "xml", "yaml", "yml"].includes(ext)) return "data";
  return "document";
}

function isAllowed(file: File): string | null {
  const ext = extensionOf(file.name);
  if (BLOCKED_EXTENSIONS.has(ext)) return `${file.name}: file type is blocked.`;
  if (file.size > MAX_FILE_SIZE) return `${file.name}: max file size is ${formatBytes(MAX_FILE_SIZE)}.`;
  const mimeOk =
    ALLOWED_MIME_TYPES.has(file.type) ||
    ALLOWED_MIME_PREFIXES.some((prefix) => file.type.startsWith(prefix)) ||
    ["ts", "tsx", "js", "jsx", "css", "html", "go", "py", "sql", "md", "yml", "yaml"].includes(ext);
  if (!mimeOk) return `${file.name}: unsupported file type.`;
  return null;
}

export function ChatComposer({
  onSend,
  onStop,
  disabled,
  pending,
  placeholder = "Tell Ironflyer what to change…",
  modelLabel = "Claude Sonnet 4.6",
}: ChatComposerProps) {
  const [value, setValue] = useState("");
  const [attachments, setAttachments] = useState<StudioAttachment[]>([]);
  const [dragging, setDragging] = useState(false);
  const [attachmentError, setAttachmentError] = useState<string | null>(null);
  const ref = useRef<HTMLTextAreaElement | null>(null);
  const fileRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    return () => {
      for (const item of attachments) {
        if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
      }
    };
  }, [attachments]);

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

  const armed = !disabled && !pending && (value.trim().length > 0 || attachments.length > 0);

  const removeAttachment = useCallback((id: string) => {
    setAttachments((current) => {
      const removed = current.find((item) => item.id === id);
      if (removed?.previewUrl) URL.revokeObjectURL(removed.previewUrl);
      return current.filter((item) => item.id !== id);
    });
  }, []);

  const addFiles = useCallback((files: FileList | File[]) => {
    setAttachmentError(null);
    const incoming = Array.from(files);
    if (incoming.length === 0) return;
    setAttachments((current) => {
      const next = [...current];
      let total = current.reduce((sum, item) => sum + item.size, 0);
      for (const file of incoming) {
        if (next.length >= MAX_FILES) {
          setAttachmentError(`Attach up to ${MAX_FILES} files per message.`);
          break;
        }
        const reason = isAllowed(file);
        if (reason) {
          setAttachmentError(reason);
          continue;
        }
        if (total + file.size > MAX_TOTAL_SIZE) {
          setAttachmentError(`Total attachment limit is ${formatBytes(MAX_TOTAL_SIZE)}.`);
          continue;
        }
        total += file.size;
        next.push({
          id:
            typeof crypto !== "undefined" && "randomUUID" in crypto
              ? crypto.randomUUID()
              : `${file.name}_${Date.now()}_${Math.random().toString(36).slice(2)}`,
          name: file.name,
          type: file.type || "application/octet-stream",
          size: file.size,
          kind: kindFor(file),
          previewUrl: file.type.startsWith("image/") ? URL.createObjectURL(file) : undefined,
        });
      }
      return next;
    });
  }, []);

  const submit = useCallback(async () => {
    const text = value.trim();
    if ((!text && attachments.length === 0) || disabled || pending) return;
    const outgoing = attachments;
    const attachmentBrief =
      outgoing.length > 0
        ? `\n\n[Attached files]\n${outgoing
            .map((item) => `- ${item.name} (${item.type || item.kind}, ${formatBytes(item.size)})`)
            .join("\n")}`
        : "";
    setValue("");
    setAttachments([]);
    setAttachmentError(null);
    await onSend(`${text || "Review the attached files."}${attachmentBrief}`, outgoing);
  }, [value, attachments, disabled, pending, onSend]);

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
      onDragEnter={(e) => {
        e.preventDefault();
        if (!disabled && !pending) setDragging(true);
      }}
      onDragOver={(e) => {
        e.preventDefault();
        if (!disabled && !pending) setDragging(true);
      }}
      onDragLeave={(e) => {
        if (e.currentTarget.contains(e.relatedTarget as Node | null)) return;
        setDragging(false);
      }}
      onDrop={(e) => {
        e.preventDefault();
        setDragging(false);
        if (disabled || pending) return;
        addFiles(e.dataTransfer.files);
      }}
    >
      <Box
        sx={{
          border: `1px solid ${
            dragging
              ? tokens.color.accent.violet
              : armed
                ? tokens.color.accent.violet + "55"
                : tokens.color.border.subtle
          }`,
          bgcolor: tokens.color.bg.inset,
          borderRadius: 1.5,
          boxShadow: dragging ? `0 0 0 3px ${tokens.color.accent.violet}26` : "none",
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
        {attachments.length > 0 && (
          <Box sx={{ borderTop: `1px solid ${tokens.color.border.subtle}`, px: 1, py: 0.85 }}>
            <Stack direction="row" spacing={0.75} useFlexGap flexWrap="wrap">
              {attachments.map((item) => (
                <Stack
                  key={item.id}
                  direction="row"
                  spacing={0.75}
                  sx={{
                    alignItems: "center",
                    bgcolor: tokens.color.bg.surfaceRaised,
                    border: `1px solid ${tokens.color.border.subtle}`,
                    borderRadius: 1,
                    maxWidth: "100%",
                    px: 0.75,
                    py: 0.5,
                  }}
                >
                  {item.previewUrl ? (
                    <Box
                      component="img"
                      src={item.previewUrl}
                      alt=""
                      sx={{ borderRadius: 0.5, height: 24, objectFit: "cover", width: 24 }}
                    />
                  ) : item.kind === "image" ? (
                    <ImageRounded sx={{ color: tokens.color.accent.violet, fontSize: 17 }} />
                  ) : (
                    <DescriptionRounded sx={{ color: tokens.color.text.secondary, fontSize: 17 }} />
                  )}
                  <Box sx={{ minWidth: 0 }}>
                    <Typography sx={{ color: tokens.color.text.primary, fontSize: 11.5, fontWeight: 800, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", maxWidth: 170 }}>
                      {item.name}
                    </Typography>
                    <Typography sx={{ color: tokens.color.text.muted, fontFamily: tokens.font.mono, fontSize: 10 }}>
                      {formatBytes(item.size)}
                    </Typography>
                  </Box>
                  <IconButton
                    size="small"
                    aria-label={`Remove ${item.name}`}
                    onClick={() => removeAttachment(item.id)}
                    sx={{ color: tokens.color.text.muted, height: 22, width: 22 }}
                  >
                    <CloseRounded sx={{ fontSize: 14 }} />
                  </IconButton>
                </Stack>
              ))}
            </Stack>
          </Box>
        )}
        {dragging && (
          <Box sx={{ px: 1.5, pb: 0.75 }}>
            <LinearProgress sx={{ bgcolor: tokens.color.bg.surfaceRaised, "& .MuiLinearProgress-bar": { bgcolor: tokens.color.accent.violet } }} />
          </Box>
        )}
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
          <input
            ref={fileRef}
            type="file"
            multiple
            hidden
            accept="image/*,.pdf,.txt,.md,.json,.csv,.xml,.yaml,.yml,.ts,.tsx,.js,.jsx,.css,.html,.go,.py,.sql,.zip"
            onChange={(event) => {
              if (event.target.files) addFiles(event.target.files);
              event.target.value = "";
            }}
          />
          <Tooltip title="Attach files or images" arrow>
            <span>
              <IconButton
                size="small"
                disabled={disabled || pending}
                onClick={() => fileRef.current?.click()}
                sx={{ color: tokens.color.text.secondary }}
                aria-label="Attach files"
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
          {attachmentError && (
            <Button
              size="small"
              onClick={() => setAttachmentError(null)}
              sx={{ color: tokens.color.accent.warning, fontSize: 10.5, minHeight: 22, px: 0.5 }}
            >
              {attachmentError}
            </Button>
          )}
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
