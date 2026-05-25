// Package runtimeclass owns the choice of sandbox isolation
// (docker / gvisor / kata / firecracker) per ARCHITECTURE_RUNTIME_SCALE.md.
// The class is a string the runtime allocator passes to the driver
// layer and to Kubernetes (Helm/k8s binds it to a real RuntimeClass
// object); per-class billing rates live here so ProfitGuard can do
// margin maths before allocation.
package runtimeclass

import "github.com/shopspring/decimal"

// Class names. Stored as plain strings so config files / CRDs can use
// them without importing the package.
const (
	ClassDocker      = "docker"
	ClassGVisor      = "gvisor"
	ClassKata        = "kata"
	ClassFirecracker = "firecracker"
)

// Rate is the per-RuntimeClass USD-per-second tick rate used by
// ProfitGuard's runtime-cost estimator. Numbers match the spec's
// "Billing Ticks" section ordering: stronger isolation costs more.
type Rate struct {
	ClassName string
	USDPerSec decimal.Decimal
}

// rateTable is the canonical per-class rate.
var rateTable = map[string]decimal.Decimal{
	ClassDocker:      decimal.RequireFromString("0.0001"),
	ClassGVisor:      decimal.RequireFromString("0.00015"),
	ClassKata:        decimal.RequireFromString("0.0002"),
	ClassFirecracker: decimal.RequireFromString("0.0003"),
}

// RateFor returns the USD/sec rate for className. Unknown classes get
// the Docker rate so a misconfiguration never produces a free
// sandbox.
func RateFor(className string) decimal.Decimal {
	if r, ok := rateTable[className]; ok {
		return r
	}
	return rateTable[ClassDocker]
}

// AllClasses returns the supported class names in stable cost order.
func AllClasses() []string {
	return []string{ClassDocker, ClassGVisor, ClassKata, ClassFirecracker}
}

// IsKnown reports whether className is one of the supported classes.
func IsKnown(className string) bool {
	_, ok := rateTable[className]
	return ok
}
