// eventToMessage — translates one ExecutionEvent (executionFeed
// subscription payload) into a StudioMessage to append to the chat
// buffer. Returns null when the event does not map to a visible bubble
// (e.g. low-level scoring updates).
//
// The full event vocabulary lives in
// apps/orchestrator/internal/execution/events.go. Keep this mapping
// table in sync; new event types should fall through to a generic
// system bubble rather than vanishing silently.
//
// A58 extends the table with the agent-reasoning vocabulary
// (agent.stage.started/action/result/completed.v1 +
// studio.refine.consumed.v1) so the chat can render a live "thinking
// out loud" thread alongside the gate verdicts.

import { EVENT_TYPES, type StudioAttachment, type StudioMessage } from "./types";

function rid(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `m_${Math.random().toString(36).slice(2)}_${Date.now()}`;
}

function asNum(v: unknown): number | null {
  if (typeof v === "number" && Number.isFinite(v)) return v;
  if (typeof v === "string") {
    const n = Number(v);
    return Number.isFinite(n) ? n : null;
  }
  return null;
}

function asStr(v: unknown): string | null {
  return typeof v === "string" && v.length > 0 ? v : null;
}

function asBool(v: unknown): boolean | null {
  if (typeof v === "boolean") return v;
  if (typeof v === "string") {
    if (v === "true") return true;
    if (v === "false") return false;
  }
  return null;
}

function asObj(v: unknown): Record<string, unknown> {
  return v && typeof v === "object" ? (v as Record<string, unknown>) : {};
}

function formatUSD(n: number): string {
  return `$${n.toFixed(n >= 1 ? 2 : 4)}`;
}

function truncate(s: string, n: number): string {
  if (s.length <= n) return s;
  return s.slice(0, Math.max(0, n - 1)) + "…";
}

// Pull a value out of a payload trying multiple key spellings (snake +
// camel) since the orchestrator's event payloads are not always
// normalised.
function pick(p: Record<string, unknown>, ...keys: string[]): unknown {
  for (const k of keys) {
    if (p[k] !== undefined && p[k] !== null) return p[k];
  }
  return undefined;
}

export interface RawExecutionEvent {
  executionID: string;
  eventType: string;
  payload: unknown;
  createdAt: string;
}

export function eventToMessage(ev: RawExecutionEvent): StudioMessage | null {
  const p = asObj(ev.payload);
  const id = `evt_${ev.executionID}_${ev.eventType}_${ev.createdAt}`;
  switch (ev.eventType) {
    case EVENT_TYPES.GateVerdict: {
      const name = asStr(p["gate"]) ?? asStr(p["name"]) ?? "Gate";
      const status = asStr(p["status"]) ?? asStr(p["verdict"]) ?? "unknown";
      const issues = asNum(p["issues"]) ?? asNum(p["issuesCount"]) ?? 0;
      return {
        id,
        role: "system",
        body: `Gate ${name}: ${status} (${issues} issue${issues === 1 ? "" : "s"})`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType, status, name, issues },
      };
    }
    case EVENT_TYPES.PatchApplied: {
      const files = Array.isArray(p["files"])
        ? (p["files"] as unknown[]).filter((f): f is string => typeof f === "string")
        : [];
      const count = asNum(p["count"]) ?? files.length;
      const sample = files.slice(0, 4).join(", ");
      const more = files.length > 4 ? `, +${files.length - 4} more` : "";
      return {
        id,
        role: "system",
        body:
          count > 0
            ? `Applied patch to ${count} file${count === 1 ? "" : "s"}${sample ? `: ${sample}${more}` : ""}`
            : `Patch applied.`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType, files },
      };
    }
    case EVENT_TYPES.RecoveryRecipeHit: {
      const recipe = asStr(p["recipe"]) ?? asStr(p["recipeID"]) ?? "recipe";
      return {
        id,
        role: "system",
        body: `Repair recipe matched (${recipe}) — attempting auto-apply...`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType, recipe },
      };
    }
    case EVENT_TYPES.ProfitGuardStop: {
      const reason = asStr(p["reason"]) ?? "ROI below threshold";
      return {
        id,
        role: "system",
        body: `Profit Guard stopped this step: ${reason}`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType, reason },
      };
    }
    case EVENT_TYPES.ExecutionSettled: {
      const spent = asNum(p["spentUSD"]) ?? asNum(p["spent"]) ?? 0;
      const margin =
        asNum(p["grossMarginPct"]) ?? asNum(p["margin"]) ?? null;
      const marginTxt = margin === null ? "n/a" : `${(margin * 100).toFixed(1)}%`;
      return {
        id,
        role: "system",
        body: `Execution settled. Spent ${formatUSD(spent)}. Margin: ${marginTxt}.`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType, spent, margin },
      };
    }
    case EVENT_TYPES.CostAdded: {
      const amt = asNum(p["amountUSD"]) ?? asNum(p["amount"]) ?? 0;
      return {
        id,
        role: "costtick",
        body: `+ ${formatUSD(amt)} added to ledger`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType, amountUSD: amt },
        costUSD: amt,
      };
    }
    case EVENT_TYPES.StudioRefine: {
      // The user-side echo lives in the chat already; we still record
      // a thin system trace so the dashboard timeline shows it.
      const msg = asStr(p["message"]) ?? "(no message)";
      return {
        id,
        role: "system",
        body: `Refinement queued: "${msg.length > 80 ? msg.slice(0, 80) + "…" : msg}"`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType },
      };
    }
    case EVENT_TYPES.Started:
      return {
        id,
        role: "system",
        body: `Execution started. Streaming events from the orchestrator…`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType },
      };
    case EVENT_TYPES.Admitted:
      return {
        id,
        role: "system",
        body: `Wallet hold placed. Execution admitted.`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType },
      };
    case EVENT_TYPES.ScoreUpdated:
      // Too chatty for the timeline; the DashboardPane shows the
      // completion score live from the bundle query instead.
      return null;

    // ── Agent reasoning vocabulary (A55 → A58) ──────────────────────
    case EVENT_TYPES.AgentStageStarted: {
      const stage = asStr(pick(p, "stage")) ?? "stage";
      const agentRole = asStr(pick(p, "agentRole", "agent_role", "role")) ?? undefined;
      return {
        id,
        role: "agent_progress",
        body: `Starting ${stage} review`,
        createdAt: ev.createdAt,
        stage,
        agentRole,
        inProgress: true,
        meta: { eventType: ev.eventType, stage, agentRole },
      };
    }
    case EVENT_TYPES.AgentStageAction: {
      const stage = asStr(pick(p, "stage")) ?? "stage";
      const action = asStr(pick(p, "action")) ?? "working";
      const target = asStr(pick(p, "target", "model", "recipe")) ?? "";
      const message = asStr(pick(p, "message", "msg")) ?? "";
      const n = asNum(pick(p, "n", "count", "files_count")) ?? null;

      let body = `${action}`;
      let variant: StudioMessage["role"] = "agent_action";
      switch (action) {
        case "model_call":
          body = target
            ? `Calling ${target} for ${stage}`
            : `Calling model for ${stage}`;
          break;
        case "patch_propose":
          body = `Proposing patch for ${stage}`;
          break;
        case "patch_apply":
          body =
            n !== null
              ? `Applying patch (${n} file${n === 1 ? "" : "s"})`
              : `Applying patch`;
          break;
        case "repair_lookup":
          body = `Looking up repair recipe for ${truncate(target || stage, 40)}`;
          break;
        case "repair_apply":
          body = `Auto-applying repair recipe`;
          break;
        case "refinement_consumed":
          variant = "refinement_ack";
          body = `Incorporating your refinement: ${truncate(message || "(no message)", 60)}`;
          break;
        case "still_working":
          body = message || `Still working on ${stage}…`;
          break;
        default:
          body = message || `${action} (${stage})`;
      }
      return {
        id,
        role: variant,
        body,
        createdAt: ev.createdAt,
        stage,
        action,
        inProgress: true,
        meta: { eventType: ev.eventType, stage, action, target, n, message },
      };
    }
    case EVENT_TYPES.AgentStageResult: {
      const stage = asStr(pick(p, "stage")) ?? "stage";
      const action = asStr(pick(p, "action")) ?? undefined;
      const success = asBool(pick(p, "success")) ?? true;
      const summary =
        asStr(pick(p, "summary", "result", "message", "detail")) ?? "";
      return {
        id,
        role: "agent_result",
        body: success ? `Done.` : `Failed.`,
        createdAt: ev.createdAt,
        stage,
        action,
        success,
        summary,
        inProgress: false,
        meta: { eventType: ev.eventType, stage, action, success, summary },
      };
    }
    case EVENT_TYPES.AgentStageCompleted: {
      const stage = asStr(pick(p, "stage")) ?? "stage";
      const status = asStr(pick(p, "status")) ?? "completed";
      const duration = asNum(pick(p, "durationMs", "duration_ms")) ?? null;
      const durTxt = duration === null ? "" : ` in ${duration}ms`;
      return {
        id,
        role: "system",
        body: `${stage} ${status}${durTxt}`,
        createdAt: ev.createdAt,
        stage,
        durationMs: duration ?? undefined,
        meta: { eventType: ev.eventType, stage, status, durationMs: duration },
      };
    }
    case EVENT_TYPES.StudioRefineConsumed: {
      return {
        id,
        role: "refinement_ack",
        body: `Your refinement was incorporated.`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType },
      };
    }
    default:
      return {
        id,
        role: "system",
        body: `${ev.eventType}`,
        createdAt: ev.createdAt,
        meta: { eventType: ev.eventType, payload: p },
      };
  }
}

export function makeUserMessage(
  body: string,
  attachments?: StudioAttachment[],
): StudioMessage {
  return {
    id: rid(),
    role: "user",
    body,
    createdAt: new Date().toISOString(),
    attachments,
  };
}

export function makeAssistantThinkingMessage(
  body: string,
  thinking?: string,
): StudioMessage {
  return {
    id: rid(),
    role: "assistant",
    body,
    thinking,
    createdAt: new Date().toISOString(),
  };
}

export function makeErrorMessage(body: string): StudioMessage {
  return {
    id: rid(),
    role: "error",
    body,
    createdAt: new Date().toISOString(),
  };
}
