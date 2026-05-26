"use client";

// ResizableSplit — horizontal split with a draggable divider. The
// divider position (in percentage of total width) is persisted to
// localStorage under SPLIT_KEY so the user's preference survives
// reloads. We clamp to [MIN_LEFT_PCT, MAX_LEFT_PCT] so neither side
// can collapse to zero.

import { Box } from "@mui/material";
import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { tokens } from "../../theme";

const SPLIT_KEY = "ironflyer.studio.split.v1";
const MIN_LEFT_PCT = 22;
const MAX_LEFT_PCT = 72;

export interface ResizableSplitProps {
  left: ReactNode;
  right: ReactNode;
  defaultLeftPct?: number;
}

export function ResizableSplit({
  left,
  right,
  defaultLeftPct = 25,
}: ResizableSplitProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const draggingRef = useRef(false);
  const [leftPct, setLeftPct] = useState<number>(defaultLeftPct);

  useEffect(() => {
    if (typeof window === "undefined") return;
    try {
      const raw = window.localStorage.getItem(SPLIT_KEY);
      if (raw) {
        const n = Number(raw);
        if (Number.isFinite(n) && n >= MIN_LEFT_PCT && n <= MAX_LEFT_PCT) {
          setLeftPct(n);
        }
      }
    } catch {
      // ignore
    }
  }, []);

  const persist = useCallback((pct: number) => {
    try {
      window.localStorage.setItem(SPLIT_KEY, String(pct));
    } catch {
      // ignore
    }
  }, []);

  const onMouseDown = useCallback(
    (e: React.MouseEvent<HTMLDivElement>) => {
      e.preventDefault();
      draggingRef.current = true;
      const onMove = (ev: MouseEvent) => {
        const el = containerRef.current;
        if (!el || !draggingRef.current) return;
        const rect = el.getBoundingClientRect();
        const pct = ((ev.clientX - rect.left) / rect.width) * 100;
        const clamped = Math.max(MIN_LEFT_PCT, Math.min(MAX_LEFT_PCT, pct));
        setLeftPct(clamped);
      };
      const onUp = () => {
        if (draggingRef.current) {
          draggingRef.current = false;
          // commit the final value to localStorage
          setLeftPct((current) => {
            persist(current);
            return current;
          });
        }
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
      };
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    },
    [persist],
  );

  return (
    <Box
      ref={containerRef}
      sx={{
        display: "flex",
        flexDirection: { xs: "column", md: "row" },
        flex: 1,
        minHeight: 0,
        minWidth: 0,
        width: "100%",
      }}
    >
      <Box
        sx={{
          minHeight: 0,
          minWidth: 0,
          width: { xs: "100%", md: `${leftPct}%` },
          height: { xs: "42%", md: "100%" },
        }}
      >
        {left}
      </Box>
      <Box
        role="separator"
        aria-orientation="vertical"
        onMouseDown={onMouseDown}
        sx={{
          alignItems: "center",
          bgcolor: tokens.color.border.subtle,
          cursor: { md: "col-resize" },
          display: { xs: "none", md: "flex" },
          height: "100%",
          justifyContent: "center",
          width: "4px",
          transition: "background 160ms ease",
          "&:hover": { bgcolor: tokens.color.accent.violet + "66" },
        }}
      />
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          minWidth: 0,
          width: { xs: "100%", md: `calc(100% - ${leftPct}% - 4px)` },
          height: { xs: "58%", md: "100%" },
        }}
      >
        {right}
      </Box>
    </Box>
  );
}
