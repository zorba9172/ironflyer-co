package compute

// Cluster Autoscaler on DOKS
// --------------------------
//
// Unlike EKS — where the operator has to install `cluster-autoscaler` (or
// Karpenter) via Helm and wire it up with IRSA — DOKS ships the
// Kubernetes cluster-autoscaler INSIDE the managed control plane. You opt
// in by setting `AutoScale: true` + `MinNodes` + `MaxNodes` on each node
// pool, which we already do in `doks.go` for both the `system` and
// `runtime` pools.
//
// DigitalOcean's autoscaler:
//   * Scales each pool independently between MinNodes and MaxNodes.
//   * Respects Kubernetes pod scheduling constraints (labels, taints,
//     tolerations, anti-affinity) without further configuration.
//   * Is upgraded with the control plane during the maintenance window
//     defined on the cluster (Sunday 03:00 UTC here).
//   * Costs nothing extra — billing is per droplet-hour for the nodes it
//     spins up, with no autoscaler overhead.
//
// So there is intentionally no Helm chart, IRSA equivalent, or extra
// resource to provision in this file. It exists as documentation so the
// next engineer doesn't go hunting for the autoscaler install that isn't
// here.
//
// If we ever want pool-of-pools / spot-style behaviour that DOKS doesn't
// support natively, the upgrade path is to install Karpenter against the
// DigitalOcean cloud-provider — but that's out of scope for the initial
// stand-up.
