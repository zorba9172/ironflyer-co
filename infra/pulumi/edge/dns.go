// Package edge owns the public-facing AWS resources: Route53 zones, ACM
// certs, CloudFront, and WAFv2. It also installs the Helm charts that need
// the hosted zone to exist (external-dns + cert-manager), since their IRSA
// policies reference the zone ARN.
package edge

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/route53"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"ironflyer/infra/pulumi/compute"
)

// NewDNS creates the Route53 hosted zone for this environment and installs
// the controllers that depend on it (external-dns + cert-manager).
//
// Zone naming:
//   - dev     → dev.ironflyer.dev
//   - staging → staging.ironflyer.dev
//   - prod-*  → ironflyer.dev (shared by all prod regions; the per-region
//     subdomains are managed as records inside that zone — only the
//     first prod stack to deploy `imports` or `create`s the zone; subsequent
//     prod stacks should be configured with a pre-existing zone via
//     `pulumi import`. The README documents this hand-off.)
func NewDNS(ctx *pulumi.Context, cfg *compute.Config, cluster *compute.Cluster) (*route53.Zone, error) {
	zone, err := route53.NewZone(ctx, "ironflyer-zone", &route53.ZoneArgs{
		Name:    pulumi.String(cfg.RootDomain),
		Comment: pulumi.String("Ironflyer " + cfg.Stack + " hosted zone"),
		Tags:    cfg.Tags(),
	})
	if err != nil {
		return nil, err
	}

	// IRSA roles that need the zone ARN -----------------------------------
	extDNSRole, err := compute.MakeIRSARole(ctx, cfg, "external-dns-sa", "kube-system", "external-dns", cluster.OIDCProviderArn, cluster.OIDCProviderURL)
	if err != nil {
		return nil, err
	}
	if err := compute.AttachInlineOutput(ctx, "external-dns-policy", extDNSRole, zone.ID().ToStringOutput().ApplyT(compute.ExternalDNSPolicy).(pulumi.StringOutput)); err != nil {
		return nil, err
	}

	cmRole, err := compute.MakeIRSARole(ctx, cfg, "cert-manager-sa", "cert-manager", "cert-manager", cluster.OIDCProviderArn, cluster.OIDCProviderURL)
	if err != nil {
		return nil, err
	}
	if err := compute.AttachInlineOutput(ctx, "cert-manager-policy", cmRole, zone.ID().ToStringOutput().ApplyT(compute.CertManagerPolicy).(pulumi.StringOutput)); err != nil {
		return nil, err
	}

	// Helm releases -------------------------------------------------------
	opts := pulumi.Provider(cluster.K8sProvider)

	if _, err := helmv3.NewRelease(ctx, "external-dns", &helmv3.ReleaseArgs{
		Chart:           pulumi.String("external-dns"),
		Version:         pulumi.String("1.14.5"),
		Namespace:       pulumi.String("kube-system"),
		CreateNamespace: pulumi.Bool(false),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://kubernetes-sigs.github.io/external-dns/"),
		},
		Values: pulumi.Map{
			"provider": pulumi.String("aws"),
			"aws": pulumi.Map{
				"region":          pulumi.String(cfg.Region),
				"zoneType":        pulumi.String("public"),
				"preferCNAME":     pulumi.Bool(false),
				"batchChangeSize": pulumi.Int(1000),
			},
			"domainFilters": pulumi.StringArray{pulumi.String(cfg.RootDomain)},
			"policy":        pulumi.String("sync"),
			"serviceAccount": pulumi.Map{
				"create": pulumi.Bool(true),
				"name":   pulumi.String("external-dns"),
				"annotations": pulumi.Map{
					"eks.amazonaws.com/role-arn": extDNSRole.Arn,
				},
			},
			"txtOwnerId": pulumi.String("ironflyer-" + cfg.Stack),
		},
	}, opts); err != nil {
		return nil, err
	}

	if _, err := helmv3.NewRelease(ctx, "cert-manager", &helmv3.ReleaseArgs{
		Chart:           pulumi.String("cert-manager"),
		Version:         pulumi.String("v1.15.1"),
		Namespace:       pulumi.String("cert-manager"),
		CreateNamespace: pulumi.Bool(true),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://charts.jetstack.io"),
		},
		Values: pulumi.Map{
			"installCRDs": pulumi.Bool(true),
			"serviceAccount": pulumi.Map{
				"create": pulumi.Bool(true),
				"name":   pulumi.String("cert-manager"),
				"annotations": pulumi.Map{
					"eks.amazonaws.com/role-arn": cmRole.Arn,
				},
			},
			"securityContext": pulumi.Map{
				"fsGroup": pulumi.Int(1001),
			},
		},
	}, opts); err != nil {
		return nil, err
	}

	return zone, nil
}

// VercelCNAMEArgs configures AddVercelCNAME. Domain is the customer-facing
// hostname (e.g. `app.eu.ironflyer.dev`); Target defaults to Vercel's
// public edge CNAME when empty.
type VercelCNAMEArgs struct {
	Domain pulumi.StringInput
	Target pulumi.StringInput
	Zone   *route53.Zone
}

// AddVercelCNAME wires a Route53 CNAME record that points
// `args.Domain` (e.g. `app.eu.ironflyer.dev`) at Vercel's edge so
// users hitting the Route53 zone resolve to the Vercel-hosted dashboard.
// Vercel's `ProjectDomain` resource handles certificate issuance on its
// side; this record just handles DNS resolution on ours.
func AddVercelCNAME(ctx *pulumi.Context, name string, args *VercelCNAMEArgs, opts ...pulumi.ResourceOption) (*route53.Record, error) {
	target := args.Target
	if target == nil {
		target = pulumi.String("cname.vercel-dns.com")
	}
	rec, err := route53.NewRecord(ctx, "vercel-cname-"+name, &route53.RecordArgs{
		ZoneId:         args.Zone.ZoneId,
		Name:           args.Domain,
		Type:           pulumi.String("CNAME"),
		Ttl:            pulumi.Int(300),
		Records:        pulumi.StringArray{target.ToStringOutput()},
		AllowOverwrite: pulumi.Bool(true),
	}, opts...)
	if err != nil {
		return nil, err
	}
	return rec, nil
}
