// Barrel for the operator dashboard panels. The /dashboard route
// composes these into the four-quadrant operator view.

export { ProfitDashboard } from "./ProfitDashboard";
export { ScaleDashboard } from "./ScaleDashboard";
export { CohortDashboard } from "./CohortDashboard";
export { BlueprintDashboard } from "./BlueprintDashboard";
export { CohortTable } from "./CohortTable";
export {
  WindowSelector,
  WINDOW_OPTIONS,
  WINDOW_LABEL,
  windowToRange,
  type DashboardWindow,
  type WindowSelectorProps,
} from "./WindowSelector";
