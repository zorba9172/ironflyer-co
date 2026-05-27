"use client";

// useChatStore — zustand-backed store for studio chat messages.
//
// Why a store (not useState): the chat buffer is read by MessageList,
// ChatComposer, SuggestionsRow, the suggestion progress, AND it needs
// to survive route changes between /p/[id] re-mounts (StrictMode in
// dev re-mounts every effect). Centralising state behind zustand:
//
//   1. Each consumer uses a selector → only that consumer re-renders
//      when the slice it cares about changes (MessageBubble can opt
//      out of the messages array entirely).
//   2. Append / merge / dedupe live in one place; the page does not
//      re-implement the localStorage juggle.
//   3. Multiple executions can coexist (keyed by executionID) without
//      the page having to clear local state on switch.
//
// localStorage is still the durable backstop — we write through on
// every mutation so a tab reload reconstructs the buffer without
// pulling from the orchestrator again.

import { create } from "zustand";
import {
  applyIncomingMessage,
  loadMessages,
  saveMessages,
} from "../../components/studio/chatStorage";
import type { StudioMessage } from "../../components/studio/types";

interface ChatState {
  byExecution: Record<string, StudioMessage[]>;
  hydrated: Record<string, true>;
  // Hydrate from localStorage. Idempotent per executionID — second
  // call is a no-op.
  hydrate(executionID: string): void;
  // Append a message that came from the server feed. Routes through
  // applyIncomingMessage to honour merge / heartbeat / dedupe rules.
  appendIncoming(executionID: string, message: StudioMessage): void;
  // Append a message generated locally (user input, optimistic ack,
  // error). Skips merge logic — local messages always render as-is.
  appendLocal(executionID: string, message: StudioMessage): void;
  // Update an existing message in place by id. Used by the SSE chat
  // stream to grow the assistant bubble as `delta` frames arrive
  // without re-appending a new message per token. No-op when the
  // message id is not in the buffer.
  updateLocal(
    executionID: string,
    messageID: string,
    patch: Partial<StudioMessage>,
  ): void;
  // Move every message from `fromID` to `toID` (used when the
  // free-chat "_" buffer is promoted into a real execution after
  // createPaidExecution resolves). No-op if `toID` already has
  // messages — we never trample a real buffer.
  adoptBuffer(fromID: string, toID: string): void;
  // Wipe an execution's buffer + clear localStorage.
  clear(executionID: string): void;
}

const EMPTY_MESSAGES: StudioMessage[] = [];

export const useChatStore = create<ChatState>((set, get) => ({
  byExecution: {},
  hydrated: {},
  hydrate: (executionID) => {
    if (!executionID) return;
    if (get().hydrated[executionID]) return;
    const messages = loadMessages(executionID);
    set((state) => ({
      byExecution: { ...state.byExecution, [executionID]: messages },
      hydrated: { ...state.hydrated, [executionID]: true },
    }));
  },
  appendIncoming: (executionID, message) => {
    if (!executionID) return;
    set((state) => {
      const prev = state.byExecution[executionID] ?? EMPTY_MESSAGES;
      const next = applyIncomingMessage(prev, message);
      if (next === prev) return state;
      saveMessages(executionID, next);
      return {
        byExecution: { ...state.byExecution, [executionID]: next },
      };
    });
  },
  appendLocal: (executionID, message) => {
    if (!executionID) return;
    set((state) => {
      const prev = state.byExecution[executionID] ?? EMPTY_MESSAGES;
      if (prev.some((m) => m.id === message.id)) return state;
      const next = [...prev, message];
      saveMessages(executionID, next);
      return {
        byExecution: { ...state.byExecution, [executionID]: next },
      };
    });
  },
  updateLocal: (executionID, messageID, patch) => {
    if (!executionID || !messageID) return;
    set((state) => {
      const prev = state.byExecution[executionID] ?? EMPTY_MESSAGES;
      const idx = prev.findIndex((m) => m.id === messageID);
      if (idx < 0) return state;
      const next = prev.slice();
      next[idx] = { ...next[idx], ...patch };
      saveMessages(executionID, next);
      return {
        byExecution: { ...state.byExecution, [executionID]: next },
      };
    });
  },
  adoptBuffer: (fromID, toID) => {
    if (!fromID || !toID || fromID === toID) return;
    set((state) => {
      const prevFrom = state.byExecution[fromID] ?? EMPTY_MESSAGES;
      if (prevFrom.length === 0) return state;
      const prevTo = state.byExecution[toID] ?? EMPTY_MESSAGES;
      if (prevTo.length > 0) return state;
      saveMessages(toID, prevFrom);
      saveMessages(fromID, []);
      const nextBy = { ...state.byExecution, [toID]: prevFrom };
      delete nextBy[fromID];
      const nextH = { ...state.hydrated, [toID]: true as const };
      delete nextH[fromID];
      return { byExecution: nextBy, hydrated: nextH };
    });
  },
  clear: (executionID) => {
    if (!executionID) return;
    set((state) => {
      if (!state.byExecution[executionID]) return state;
      const nextBy = { ...state.byExecution };
      delete nextBy[executionID];
      const nextH = { ...state.hydrated };
      delete nextH[executionID];
      // Wipe the storage backstop too.
      saveMessages(executionID, []);
      return { byExecution: nextBy, hydrated: nextH };
    });
  },
}));

// Selector helpers — keep selector identity stable so consumers can
// useChatStore(selectMessages(executionID)) without re-creating the
// selector on every render.
export function selectMessages(executionID: string) {
  return (state: ChatState): StudioMessage[] =>
    state.byExecution[executionID] ?? EMPTY_MESSAGES;
}
