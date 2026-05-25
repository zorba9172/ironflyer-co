package profitguard

// Decision is the verdict returned by Guard.Decide. The runtime acts
// on Action; Reason / RecommendedProvider / ShouldDowngradeModelTier /
// Metadata are advisory channels the caller uses to actually carry
// out the decision (pick a different provider, drop to Sonnet, surface
// the reason to the user, etc.).
//
// ExpectedMarginPct is the margin Decide computed for this step
// against UserBudgetUSD; surfacing it lets dashboards plot
// "decisions vs. margin headroom" without recomputing.
type Decision struct {
	Action                   Action
	Reason                   string
	ExpectedMarginPct        float64
	RecommendedProvider      string
	ShouldDowngradeModelTier bool
	Metadata                 map[string]any
}
