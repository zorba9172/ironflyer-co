// Package buildinfo exposes the bridge service's build identity. Both
// the /healthz handler and structured logs read from here so version
// drift never lies about what's running.
package buildinfo

// Version is the human-readable bridge version. Override at build time
// via -ldflags '-X ironflyer/clients/scrcpy-bridge/internal/buildinfo.Version=...'.
var Version = "dev"

// Component identifies the service in logs and metrics.
const Component = "scrcpy-bridge"
