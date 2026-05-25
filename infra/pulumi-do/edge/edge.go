// Package edge owns the public-facing layer of the DigitalOcean stack:
// cert-manager (TLS), sealed-secrets (in-cluster GitOps-friendly secret
// distribution), ingress-nginx (the cluster's LoadBalancer front door),
// Cloudflare (authoritative DNS + WAF + zone hardening), and the
// Vercel-hosted dashboard project.
//
// The package is the DO mirror of `ironflyer/infra/pulumi/edge` (AWS). The
// shape of the public API matches by design — the only differences are the
// cloud-specific resources:
//
//   - Cloudflare records / WAF in place of Route53 + ACM + CloudFront + WAFv2.
//   - DO LoadBalancer service annotations in place of NLB target groups.
//   - cert-manager + Let's Encrypt HTTP01 in place of ACM-issued certs
//     attached to CloudFront.
//   - sealed-secrets controller in place of AWS Secrets Manager + IRSA.
//
// Provisioning order (must run after compute + data):
//
//	cert-manager   → Helm release, installs CRDs, owns the ClusterIssuer.
//	sealed-secrets → Helm release of the controller (kubeseal CLI uses
//	                 `kubeseal --fetch-cert` once after this lands; see
//	                 sealed_secrets.go).
//	ingress-nginx  → Helm release. Provisions the DO LoadBalancer via the
//	                 documented `service.beta.kubernetes.io/do-loadbalancer-*`
//	                 annotations.
//	cloudflare     → DNS records, zone settings, WAF ruleset, page rules.
//	                 A records target the ingress LB IP; CNAME records target
//	                 Vercel.
//	vercel         → Project + production domain + env wiring. Gated by
//	                 cfg.VercelEnabled.
//
// Each step takes the same `Inputs` struct so main.go composes them in a
// single call (`edge.Provision(ctx, edge.Inputs{...})`).
package edge

import (
	"strings"

	"github.com/pulumi/pulumi-digitalocean/sdk/v4/go/digitalocean"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"ironflyer/infra/pulumi-do/compute"
	"ironflyer/infra/pulumi-do/data"
)

// Inputs is the single struct main.go passes into Provision. Every field
// is required; the data package's `*Outputs` may carry zero-valued Output
// fields while the sibling agent is still scaffolding — none of the edge
// code dereferences them eagerly, so that's safe.
type Inputs struct {
	Config      *compute.Config
	Network     *compute.Network
	Cluster     *digitalocean.KubernetesCluster
	K8sProvider *kubernetes.Provider
	Data        *data.Outputs
}

// Provision installs the entire edge layer in the documented order.
// Order matters: ingress-nginx must exist before cert-manager's HTTP01
// solver can answer ACME challenges, and the Cloudflare DNS records depend
// on the ingress LoadBalancer IP being ready.
func Provision(ctx *pulumi.Context, in Inputs) error {
	cmRel, err := newCertManager(ctx, in)
	if err != nil {
		return err
	}
	ssRel, err := newSealedSecrets(ctx, in)
	if err != nil {
		return err
	}
	lbIP, ingRel, err := newIngressNginx(ctx, in, cmRel, ssRel)
	if err != nil {
		return err
	}
	if err := newClusterIssuer(ctx, in, cmRel, ingRel); err != nil {
		return err
	}
	if err := newCloudflare(ctx, in, lbIP); err != nil {
		return err
	}
	if err := newVercel(ctx, in); err != nil {
		return err
	}

	// Re-export the bits operators want to see at the top of
	// `pulumi stack output` without flipping through Helm releases.
	ctx.Export("ingressLoadBalancerIP", lbIP)
	ctx.Export("edgeRootDomain", pulumi.String(rootDomain(in.Config)))
	ctx.Export("edgeAcmeEmail", pulumi.String(acmeEmail(in.Config)))

	return nil
}

// rootDomain is the apex used for Cloudflare zone lookups + ACME
// registration. When VercelDomain is set (e.g. `app.ironflyer.dev`) and
// RootDomain is empty we derive the apex by trimming the leading label.
func rootDomain(cfg *compute.Config) string {
	if cfg.RootDomain != "" {
		return cfg.RootDomain
	}
	if cfg.VercelDomain == "" {
		return ""
	}
	parts := strings.SplitN(cfg.VercelDomain, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return cfg.VercelDomain
}

// acmeEmail is the registration email Let's Encrypt sends expiry +
// rotation notices to. Falls back to `admin@<rootDomain>` when the
// operator did not set ironflyer:acmeEmail explicitly — keeps prod stacks
// from registering ACME accounts under a placeholder when an operator
// forgot to set the config.
func acmeEmail(cfg *compute.Config) string {
	root := rootDomain(cfg)
	if root == "" {
		return "admin@ironflyer.dev"
	}
	return "admin@" + root
}

// apiHostname returns the orchestrator API hostname for this stack.
// `api.<root>` matches the AWS edge naming convention so a Vercel
// dashboard built for either cloud sees the same env-var values.
func apiHostname(cfg *compute.Config) string {
	root := rootDomain(cfg)
	if root == "" {
		return ""
	}
	return "api." + root
}

// runtimeHostname is the workspace runtime's public hostname. Separate
// from the orchestrator so we can rate-limit and cache it independently
// at the Cloudflare edge.
func runtimeHostname(cfg *compute.Config) string {
	root := rootDomain(cfg)
	if root == "" {
		return ""
	}
	return "runtime." + root
}

// docsHostname is the public docs site hostname. Cloudflare aggressively
// caches it (1d TTL) so the upstream origin can be either a Vercel
// project or a Spaces-backed static bucket.
func docsHostname(cfg *compute.Config) string {
	root := rootDomain(cfg)
	if root == "" {
		return ""
	}
	return "docs." + root
}

// appHostname is the dashboard hostname. Prefers the explicit
// VercelDomain when set (it may include a stack-specific prefix like
// `app.staging.ironflyer.dev`), otherwise composes `app.<root>`.
func appHostname(cfg *compute.Config) string {
	if cfg.VercelDomain != "" {
		return cfg.VercelDomain
	}
	root := rootDomain(cfg)
	if root == "" {
		return ""
	}
	return "app." + root
}
