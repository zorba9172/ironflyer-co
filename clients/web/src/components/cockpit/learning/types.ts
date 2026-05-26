// Shared shapes for the Learning System Dashboard panels.
//
// These mirror the expected GraphQL `LearningDashboard` type — the
// resolver is built in parallel; until it lands the page renders
// against a `placeholderLearning()` zero-state shape so panels can
// surface their "report not wired" affordance.
//
// Sentinel rules (until the backend defines its own):
//   - outcomeEventsToday/AllTime = 0       → never recorded an outcome
//   - reuseRateLast7d = -1                  → no PreflightDecisions yet
//   - banditConfidence = -1                 → bandit hasn't trained
//   - gateFailureRates = []                 → no gate runs recorded
//   - blueprintSuccessRates = []            → no blueprint executions
//   - weaknesses = []                       → analyzer hasn't run
//   - lastIndexedAt = null                  → never indexed

export interface GateFailureRate {
  gate: string;
  failureRate: number;
  sampleSize: number;
}

export interface BlueprintSuccessRate {
  blueprintID: string;
  blueprintName: string;
  successRate: number;
  avgMargin: number;
  // Optional sample size — when the backend exposes it the scatter
  // sizes its points; otherwise every point uses a constant size.
  sampleSize?: number;
}

export interface Weakness {
  dimension: string;
  description: string;
  severity: string; // "high" | "medium" | "low"
  suggestedAction: string;
  // Optional supporting evidence (paths, sample IDs) — surfaced when
  // the operator expands the row. Absent until the resolver ships.
  evidence?: string[];
}

// Per-hour outcome event count for the LearningPulsePanel sparkline.
// Not part of the V1 GraphQL shape — the panel synthesizes a flat
// zero series from `outcomeEventsToday` until the resolver exposes
// per-hour bins.
export interface HourlyOutcomeBucket {
  hour: string; // ISO timestamp, top-of-hour
  count: number;
}

// Week-over-week delta block for the LearningRecapPanel.
// Until the resolver exposes prior-7d aggregates we render the
// current-week numbers with a `null` delta and a neutral indicator.
export interface WeekDelta {
  completionScoreDelta: number | null;
  marginDelta: number | null;
  reuseRateDelta: number | null;
  repairRecipeHitsDelta: number | null;
}

export interface LearningDashboardShape {
  outcomeEventsToday: number;
  outcomeEventsAllTime: number;
  reuseRateLast7d: number;
  repairRecipeHitsLast7d: number;
  banditConfidence: number;
  averageCompletionScore: number;
  averageMarginPctLast7d: number;
  gateFailureRates: GateFailureRate[];
  blueprintSuccessRates: BlueprintSuccessRate[];
  weaknesses: Weakness[];
  lastIndexedAt: string | null;
  // Local-only fields (synthesized by the page when the resolver
  // doesn't yet expose them).
  hourlyOutcomes?: HourlyOutcomeBucket[];
  weekDelta?: WeekDelta;
}

// Live placeholder builder used while the orchestrator's GraphQL
// resolver is still a stub. Returns the "report not wired" sentinel
// state so every panel can demonstrate its empty pattern without
// crashing.
export function placeholderLearning(): LearningDashboardShape {
  return {
    outcomeEventsToday: 0,
    outcomeEventsAllTime: 0,
    reuseRateLast7d: -1,
    repairRecipeHitsLast7d: 0,
    banditConfidence: -1,
    averageCompletionScore: 0,
    averageMarginPctLast7d: 0,
    gateFailureRates: [],
    blueprintSuccessRates: [],
    weaknesses: [],
    lastIndexedAt: null,
    hourlyOutcomes: [],
    weekDelta: {
      completionScoreDelta: null,
      marginDelta: null,
      reuseRateDelta: null,
      repairRecipeHitsDelta: null,
    },
  };
}

// Severity rank used to sort the Weaknesses list. Unknown severities
// sort to the bottom.
export function severityRank(severity: string): number {
  switch (severity.toLowerCase()) {
    case "high":
    case "critical":
      return 3;
    case "medium":
    case "warning":
      return 2;
    case "low":
    case "info":
      return 1;
    default:
      return 0;
  }
}
