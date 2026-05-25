package deploy

// Event-type constants written into deploy_events. Keeping them as
// constants (not free strings) means the resolver, subscribers, and
// dashboards can all switch on a stable vocabulary.
const (
	EventPlanned          = "planned"
	EventPreviewBuilding  = "preview_building"
	EventPreviewReady     = "preview_ready"
	EventApprovalRequest  = "approval_requested"
	EventApprovalDecided  = "approval_decided"
	EventApprovalExpired  = "approval_expired"
	EventPromoting        = "promoting"
	EventPromoted         = "promoted"
	EventRolledBack       = "rolled_back"
	EventFailed           = "failed"
	EventCancelled        = "cancelled"
	EventCostRecorded     = "cost_recorded"
	EventProfitGuardBlock = "profit_guard_block"
)
