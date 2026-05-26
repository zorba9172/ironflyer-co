// Shared shapes for the Code Health Dashboard panels.
//
// These mirror `core/orchestrator/internal/business/dashboards/health.go`
// (HealthDashboard struct). The orchestrator's GraphQL schema does not
// yet expose `healthDashboard { ... }` — the panels render against a
// stub shape until that resolver is wired. When the GraphQL operation
// lands, swap the local hooks for the codegen'd Apollo hook and remove
// the placeholder data.
//
// Sentinel rules (must match health.go):
//   - reuseRate = -1               → no PreflightDecisions recorded
//   - dedupRate = -1               → no jscpd / dupl report wired
//   - deadcodeCount = -1           → no knip / ts-prune report wired
//   - complexityHistogram = null   → no gocognit / sonarjs report wired
//   - dependencyCycles = -1        → no depcruiser / madge report wired
//   - locPerCapability = 0         → first-run; no patches landed yet
//   - atlasCapabilityCount = 0     → Atlas has not indexed yet

export interface HealthDashboardShape {
  projectId: string;
  asOf: string; // ISO timestamp
  reuseRate: number;
  dedupRate: number;
  deadcodeCount: number;
  complexityHistogram: number[] | null;
  dependencyCycles: number;
  locPerCapability: number;
  atlasCapabilityCount: number;
  lastIndexedAt: string; // ISO timestamp, zero-value when never indexed
}

// Per-directory dup rate (for the heatmap). Not exposed by the V22
// HealthDashboard struct yet — projected from the jscpd report in a
// follow-up. The shape is what the heatmap consumes.
export interface DuplicationDirRow {
  directory: string;
  dupPct: number;
  // Lines of code in this directory; used for the heatmap cell size.
  loc: number;
}

// Per-route bundle weight row (size-limit + @next/bundle-analyzer).
export interface BundleRouteRow {
  route: string;
  totalKB: number;
  firstLoadKB: number;
  perChunkKB: number;
}

// Architecture manifest row exposed under a future
// `architecture { layers, rules }` GraphQL field.
export interface ArchitectureLayer {
  name: string;
  packages: string[];
}
export interface ArchitectureRule {
  from: string;
  to: string;
  allow: boolean;
}

// Live placeholder builder used while the orchestrator's GraphQL
// resolver is still a stub. Returns the "tool not wired" sentinel
// state from health.go so every panel can demonstrate its empty
// pattern without crashing.
export function placeholderHealth(projectId = "demo"): HealthDashboardShape {
  return {
    projectId,
    asOf: new Date().toISOString(),
    reuseRate: -1,
    dedupRate: -1,
    deadcodeCount: -1,
    complexityHistogram: null,
    dependencyCycles: -1,
    locPerCapability: 0,
    atlasCapabilityCount: 0,
    lastIndexedAt: "0001-01-01T00:00:00Z",
  };
}
