package securityreport

import "errors"

// Sentinel errors. The resolver layer maps ErrExecutionNotFound to
// 404 and ErrFindingSourceUnconfigured to a graceful "no security
// data yet" so the dashboard renders an empty-state instead of an
// error chip on dev boxes.
var (
	ErrExecutionNotFound         = errors.New("securityreport: execution not found")
	ErrFindingSourceUnconfigured = errors.New("securityreport: no FindingSource wired")
)
