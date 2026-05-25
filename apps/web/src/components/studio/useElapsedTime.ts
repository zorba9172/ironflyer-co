"use client";

// useElapsedTime — 1Hz ticker for the studio context bar.
//
// Returns a mm:ss (or h:mm:ss) string updated once per second while the
// execution is alive. The hook is intentionally tiny so multiple panes
// can share it without dragging in StudioContextBar's freeze-on-terminal
// logic. When a caller needs the freeze behaviour it should pass a
// frozen "now" instead and skip the interval.
//
// Brief-mandated module — A62-v2 brief, file ownership list.

import { useEffect, useState } from "react";

export function useElapsedTime(startISO: string): string {
  const [elapsed, setElapsed] = useState<string>(() => format(startISO));
  useEffect(() => {
    setElapsed(format(startISO));
    const t = setInterval(() => setElapsed(format(startISO)), 1000);
    return () => clearInterval(t);
  }, [startISO]);
  return elapsed;
}

function format(startISO: string): string {
  const startTs = Date.parse(startISO);
  const safeStart = Number.isFinite(startTs) ? startTs : Date.now();
  const diffMs = Date.now() - safeStart;
  const s = Math.max(0, Math.floor(diffMs / 1000));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${pad(m)}:${pad(sec)}`;
}
