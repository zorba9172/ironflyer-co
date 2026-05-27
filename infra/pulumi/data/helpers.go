// Package data provisions the Ironflyer DigitalOcean data layer:
// managed Postgres (with pgvector), managed Valkey/Redis, Spaces
// buckets + access keys, in-cluster Kubernetes Secrets for the
// orchestrator, and optional observability add-ons.
//
// It mirrors the AWS data layer at infra/pulumi/data/ but uses
// DigitalOcean's managed services. The compute/ + edge/ sibling
// packages own the VPC, DOKS, Cloudflare, and Vercel surfaces; this
// package consumes their outputs through the Inputs struct.
package data

import (
	"ironflyer/infra/pulumi-do/compute"
)

// IfHA returns hi when the stack is configured for high availability,
// otherwise lo. Used for managed-database node counts.
func IfHA[T any](cfg *compute.Config, hi, lo T) T {
	if cfg != nil && cfg.EnableHA {
		return hi
	}
	return lo
}

// spacesEndpoint derives the Spaces S3-compatible endpoint hostname for
// the configured region. Spaces endpoints follow the pattern
// `<region>.digitaloceanspaces.com` (or `<region>.cdn.digitaloceanspaces.com`
// when CDN is enabled — we use the non-CDN form for backend traffic).
func spacesEndpoint(region string) string {
	return region + ".digitaloceanspaces.com"
}
