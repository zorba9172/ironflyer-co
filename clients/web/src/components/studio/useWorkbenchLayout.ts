"use client";

// useWorkbenchLayout — persisted UI state for the studio shell.
//
// One hook owns:
//   • Which "primary" pane is shown in the center stage (preview, code,
//     files, dashboard).
//   • Whether the left rail, the right rail (chat) and the bottom
//     dock are open or collapsed.
//   • The currently active bottom-dock tab (patches / logs / changes).
//   • A "focus mode" flag that hides every rail and dock so the center
//     stage owns the entire surface.
//
// State is persisted to localStorage keyed by projectID so different
// projects can have different layouts and reloading the page never
// loses the operator's preference. The hook is SSR-safe — reads only
// happen inside useEffect.

import { useCallback, useEffect, useMemo, useState } from "react";

// Center stage primary pane. The chat lives in its own right rail and
// the bottom dock owns patches/logs — those aren't valid primaries.
// "mobile" renders the preview at phone viewport width so the operator
// can sanity-check responsive layout without opening dev tools.
export type WorkbenchPrimary =
  | "preview"
  | "mobile"
  | "code"
  | "files"
  | "dashboard";

// Bottom dock tab. "terminal" opens a real PTY into the workspace
// (xterm.js + runtime WebSocket), matching VS Code's bottom panel
// layout of Problems / Output / Terminal / Debug.
export type WorkbenchDockTab = "patches" | "logs" | "changes" | "terminal";

export interface WorkbenchLayoutState {
  primary: WorkbenchPrimary;
  leftOpen: boolean;
  rightOpen: boolean;
  dockOpen: boolean;
  dockTab: WorkbenchDockTab;
  dockHeight: number; // px
  focus: boolean;
}

// DEFAULT_STATE — landing layout for a fresh project.
//
// The Studio now lands on VS Code by default. Preview, mobile, files
// and dashboard stay one shortcut away, but the operator's first
// surface is the professional workspace where code and terminal work
// happen.
const DEFAULT_STATE: WorkbenchLayoutState = {
  primary: "code",
  leftOpen: true,
  rightOpen: true,
  dockOpen: false,
  dockTab: "patches",
  dockHeight: 240,
  focus: false,
};

const MIN_DOCK_HEIGHT = 140;
const MAX_DOCK_HEIGHT = 560;

function storageKey(projectID: string): string {
  return `ironflyer.studio.layout.v3:${projectID || "_"}`;
}

function isValid(raw: unknown): raw is WorkbenchLayoutState {
  if (!raw || typeof raw !== "object") return false;
  const s = raw as Record<string, unknown>;
  const validPrimary =
    s.primary === "preview" ||
    s.primary === "mobile" ||
    s.primary === "code" ||
    s.primary === "files" ||
    s.primary === "dashboard";
  const validDockTab =
    s.dockTab === "patches" ||
    s.dockTab === "logs" ||
    s.dockTab === "changes" ||
    s.dockTab === "terminal";
  return (
    validPrimary &&
    validDockTab &&
    typeof s.leftOpen === "boolean" &&
    typeof s.rightOpen === "boolean" &&
    typeof s.dockOpen === "boolean" &&
    typeof s.focus === "boolean" &&
    typeof s.dockHeight === "number"
  );
}

export interface UseWorkbenchLayoutResult extends WorkbenchLayoutState {
  setPrimary: (next: WorkbenchPrimary) => void;
  toggleLeft: () => void;
  toggleRight: () => void;
  toggleDock: () => void;
  toggleFocus: () => void;
  setDockTab: (tab: WorkbenchDockTab) => void;
  setDockHeight: (px: number) => void;
}

export function useWorkbenchLayout(
  projectID: string,
): UseWorkbenchLayoutResult {
  const [state, setState] = useState<WorkbenchLayoutState>(DEFAULT_STATE);

  // Hydrate from localStorage on mount + when projectID flips.
  useEffect(() => {
    if (typeof window === "undefined") return;
    try {
      const raw = window.localStorage.getItem(storageKey(projectID));
      if (!raw) {
        setState(DEFAULT_STATE);
        return;
      }
      const parsed: unknown = JSON.parse(raw);
      if (isValid(parsed)) {
        setState({
          ...DEFAULT_STATE,
          ...parsed,
          dockHeight: Math.max(
            MIN_DOCK_HEIGHT,
            Math.min(MAX_DOCK_HEIGHT, parsed.dockHeight),
          ),
        });
      } else {
        setState(DEFAULT_STATE);
      }
    } catch {
      setState(DEFAULT_STATE);
    }
  }, [projectID]);

  // Persist on every change. Debounce is unnecessary — these flips are
  // human-driven and infrequent.
  useEffect(() => {
    if (typeof window === "undefined") return;
    try {
      window.localStorage.setItem(storageKey(projectID), JSON.stringify(state));
    } catch {
      // localStorage may be unavailable (private mode etc.); silently
      // skip — the layout will reset on next mount.
    }
  }, [projectID, state]);

  const setPrimary = useCallback((next: WorkbenchPrimary) => {
    setState((s) => (s.primary === next ? s : { ...s, primary: next }));
  }, []);
  const toggleLeft = useCallback(() => {
    setState((s) => ({ ...s, leftOpen: !s.leftOpen, focus: false }));
  }, []);
  const toggleRight = useCallback(() => {
    setState((s) => ({ ...s, rightOpen: !s.rightOpen, focus: false }));
  }, []);
  const toggleDock = useCallback(() => {
    setState((s) => ({ ...s, dockOpen: !s.dockOpen, focus: false }));
  }, []);
  const toggleFocus = useCallback(() => {
    setState((s) => ({ ...s, focus: !s.focus }));
  }, []);
  const setDockTab = useCallback((tab: WorkbenchDockTab) => {
    setState((s) => ({ ...s, dockTab: tab, dockOpen: true }));
  }, []);
  const setDockHeight = useCallback((px: number) => {
    setState((s) => ({
      ...s,
      dockHeight: Math.max(MIN_DOCK_HEIGHT, Math.min(MAX_DOCK_HEIGHT, px)),
    }));
  }, []);

  return useMemo<UseWorkbenchLayoutResult>(
    () => ({
      ...state,
      setPrimary,
      toggleLeft,
      toggleRight,
      toggleDock,
      toggleFocus,
      setDockTab,
      setDockHeight,
    }),
    [
      state,
      setPrimary,
      toggleLeft,
      toggleRight,
      toggleDock,
      toggleFocus,
      setDockTab,
      setDockHeight,
    ],
  );
}
