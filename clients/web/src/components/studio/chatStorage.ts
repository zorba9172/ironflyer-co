// chatStorage — localStorage-backed buffer keyed by executionID.
//
// Why: a Studio session must survive a tab reload. The orchestrator
// already persists the event log, but re-replaying the entire log on
// every mount is expensive and would re-trigger animations. We snapshot
// the rendered StudioMessage list locally and dedupe-merge incoming
// events.
//
// We cap the buffer at MAX_MESSAGES per execution so a runaway session
// can't blow out localStorage. Older entries fall off in insertion
// order.
//
// A58 adds two helpers on top of the basic append/merge:
//
//   mergeAgentResult     — when an agent.stage.result.v1 lands, walk
//                          back to find the matching in-flight
//                          agent_action/agent_progress bubble and flip
//                          it from "in progress" to "complete" in
//                          place, instead of appending a new bubble.
//   dedupeHeartbeat      — collapse `still_working` heartbeats so we
//                          render at most one per 5 seconds per stage,
//                          and replace a prior in-flight action of the
//                          same stage with the heartbeat body when it
//                          is genuinely newer.

import type { StudioMessage } from "./types";

const KEY_PREFIX = "ironflyer.studio.chat.v1.";
const MAX_MESSAGES = 500;

// Merge window: a result event may arrive seconds after its action.
// 60s is generous but matches the orchestrator's stage timeout floor.
const MERGE_WINDOW_MS = 60_000;
// Heartbeat throttle: render at most one still_working per 5s/stage.
const HEARTBEAT_THROTTLE_MS = 5_000;

function key(executionID: string): string {
  return `${KEY_PREFIX}${executionID}`;
}

export function loadMessages(executionID: string): StudioMessage[] {
  if (typeof window === "undefined") return [];
  if (!executionID) return [];
  try {
    const raw = window.localStorage.getItem(key(executionID));
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    // Light validation: every entry must have id, role, body, createdAt.
    return parsed.filter(
      (m): m is StudioMessage =>
        !!m &&
        typeof (m as StudioMessage).id === "string" &&
        typeof (m as StudioMessage).role === "string" &&
        typeof (m as StudioMessage).body === "string" &&
        typeof (m as StudioMessage).createdAt === "string",
    );
  } catch {
    return [];
  }
}

export function saveMessages(executionID: string, messages: StudioMessage[]): void {
  if (typeof window === "undefined") return;
  if (!executionID) return;
  const trimmed =
    messages.length > MAX_MESSAGES
      ? messages.slice(messages.length - MAX_MESSAGES)
      : messages;
  try {
    window.localStorage.setItem(key(executionID), JSON.stringify(trimmed));
  } catch {
    // QuotaExceededError or private mode — silently drop. The
    // orchestrator's event log is the durable source of truth.
  }
}

export function appendMessage(
  prev: StudioMessage[],
  next: StudioMessage,
): StudioMessage[] {
  // Dedupe by id — execution events arrive both via the subscription
  // and (after a reload) via the persisted buffer; same id wins once.
  if (prev.some((m) => m.id === next.id)) return prev;
  return [...prev, next];
}

export function mergeMessages(
  prev: StudioMessage[],
  incoming: StudioMessage[],
): StudioMessage[] {
  if (incoming.length === 0) return prev;
  const seen = new Set(prev.map((m) => m.id));
  const out = prev.slice();
  for (const m of incoming) {
    if (!seen.has(m.id)) {
      out.push(m);
      seen.add(m.id);
    }
  }
  return out;
}

// timeDeltaMs returns the absolute difference in ms between two ISO
// timestamps, or +Infinity if either is unparseable.
function timeDeltaMs(a: string, b: string): number {
  const ta = Date.parse(a);
  const tb = Date.parse(b);
  if (!Number.isFinite(ta) || !Number.isFinite(tb)) return Infinity;
  return Math.abs(ta - tb);
}

// mergeAgentResult — when an agent_result lands, walk back from the
// tail of the buffer looking for the matching in-flight agent_action
// (same stage, same action when both are present) within the merge
// window. If found, mutate it in place: inProgress=false, copy
// success/summary, swap the body for a terminal "Done."/"Failed.".
//
// If no matching action exists (e.g. event arrived without its
// counterpart), the result is appended as a standalone bubble so the
// user still sees it.
export function mergeAgentResult(
  buffer: StudioMessage[],
  result: StudioMessage,
): StudioMessage[] {
  if (result.role !== "agent_result") {
    return appendMessage(buffer, result);
  }
  // Dedupe by id first.
  if (buffer.some((m) => m.id === result.id)) return buffer;

  for (let i = buffer.length - 1; i >= 0; i--) {
    const m = buffer[i];
    if (m.role !== "agent_action" && m.role !== "agent_progress") continue;
    if (!m.inProgress) continue;
    if (m.stage !== result.stage) continue;
    // If both sides declare an action, they must match. If one side
    // omits, we still allow the merge (a started/result pair without
    // an action verb is common).
    if (m.action && result.action && m.action !== result.action) continue;
    if (timeDeltaMs(m.createdAt, result.createdAt) > MERGE_WINDOW_MS) {
      // Too old — stop walking, the action will never be merged.
      break;
    }

    const merged: StudioMessage = {
      ...m,
      inProgress: false,
      success: result.success,
      summary: result.summary || m.summary,
      body: m.body, // keep the verb on top
      meta: { ...m.meta, mergedResultId: result.id },
    };
    const out = buffer.slice();
    out[i] = merged;
    return out;
  }

  // No matching in-flight action — surface the result on its own.
  return appendMessage(buffer, result);
}

// dedupeHeartbeat — throttle `still_working` heartbeats. If the tail
// of the buffer already has an in-flight action for the same stage and
// the new heartbeat lands within HEARTBEAT_THROTTLE_MS, drop it (or
// replace the body in place if the heartbeat carries a fresher status
// message).
//
// For other in-flight transitions (e.g. model_call → patch_propose for
// the same stage) we DO append — those are meaningfully different
// actions and the user should see them.
export function dedupeHeartbeat(
  buffer: StudioMessage[],
  heartbeat: StudioMessage,
): StudioMessage[] {
  if (heartbeat.action !== "still_working") {
    return appendMessage(buffer, heartbeat);
  }
  // Dedupe by id first.
  if (buffer.some((m) => m.id === heartbeat.id)) return buffer;

  for (let i = buffer.length - 1; i >= 0; i--) {
    const m = buffer[i];
    if (m.role !== "agent_action" && m.role !== "agent_progress") continue;
    if (!m.inProgress) continue;
    if (m.stage !== heartbeat.stage) continue;

    const dt = timeDeltaMs(m.createdAt, heartbeat.createdAt);
    if (dt < HEARTBEAT_THROTTLE_MS) {
      // Within the throttle window — drop the heartbeat entirely if
      // the existing in-flight message is also a heartbeat (true
      // duplicate); otherwise keep the prior action and skip.
      return buffer;
    }
    // Outside the throttle window — update the prior heartbeat in
    // place with the fresher body so the stage shows liveness without
    // adding another bubble.
    if (m.action === "still_working") {
      const refreshed: StudioMessage = {
        ...m,
        body: heartbeat.body,
        createdAt: heartbeat.createdAt,
      };
      const out = buffer.slice();
      out[i] = refreshed;
      return out;
    }
    // Different in-flight action (e.g. model_call still running) — let
    // the heartbeat through as a separate row so the user sees that
    // the agent is alive.
    break;
  }

  return appendMessage(buffer, heartbeat);
}

// applyIncomingMessage routes an incoming event-derived message
// through the right merge helper. Other studio code can stay simple
// and call this single entry point.
export function applyIncomingMessage(
  buffer: StudioMessage[],
  next: StudioMessage,
): StudioMessage[] {
  if (next.role === "agent_result") {
    return mergeAgentResult(buffer, next);
  }
  if (next.role === "agent_action" && next.action === "still_working") {
    return dedupeHeartbeat(buffer, next);
  }
  return appendMessage(buffer, next);
}
