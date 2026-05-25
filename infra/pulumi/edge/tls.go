package edge

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/acm"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/route53"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"ironflyer/infra/pulumi/compute"
)

// Certs bundles the two ACM certs the platform needs.
type Certs struct {
	RegionalArn pulumi.StringOutput
	UsEast1Arn  pulumi.StringOutput
}

// NewTLS issues two DNS-validated ACM certificates:
//
//   - A regional cert (same region as the EKS cluster) used by the ALB
//     in front of the orchestrator's Ingress.
//   - A us-east-1 cert (mandatory for CloudFront) used by the CDN.
//
// Both cover wildcard + apex. DNS-01 validation records are created
// automatically against the zone we own.
func NewTLS(ctx *pulumi.Context, cfg *compute.Config, zone *route53.Zone, usEast1 *aws.Provider) (*Certs, error) {
	domains := pulumi.StringArray{
		pulumi.String("*." + cfg.RootDomain),
	}

	regional, regionalArn, err := issueCert(ctx, cfg, zone, "regional", domains, nil)
	if err != nil {
		return nil, err
	}
	_ = regional

	cf, cfArn, err := issueCert(ctx, cfg, zone, "cloudfront", domains, pulumi.Provider(usEast1))
	if err != nil {
		return nil, err
	}
	_ = cf

	return &Certs{
		RegionalArn: regionalArn,
		UsEast1Arn:  cfArn,
	}, nil
}

func issueCert(ctx *pulumi.Context, cfg *compute.Config, zone *route53.Zone, name string, sans pulumi.StringArray, providerOpt pulumi.ResourceOption) (*acm.Certificate, pulumi.StringOutput, error) {
	opts := []pulumi.ResourceOption{}
	if providerOpt != nil {
		opts = append(opts, providerOpt)
	}

	cert, err := acm.NewCertificate(ctx, "cert-"+name, &acm.CertificateArgs{
		DomainName:              pulumi.String(cfg.RootDomain),
		SubjectAlternativeNames: sans,
		ValidationMethod:        pulumi.String("DNS"),
		Tags: cfg.TagsWith(map[string]string{
			"Name":    "ironflyer-" + cfg.Stack + "-" + name,
			"Purpose": name,
		}),
	}, opts...)
	if err != nil {
		return nil, pulumi.StringOutput{}, err
	}

	// Create one Route53 record per validation option (one per SAN, deduped).
	// Pulumi can't loop a dynamic Output[List] at construction time, so we
	// instead create the records inline via an Apply that fans out.
	validation := cert.DomainValidationOptions.ApplyT(func(opts []acm.CertificateDomainValidationOption) []map[string]string {
		seen := make(map[string]bool, len(opts))
		out := make([]map[string]string, 0, len(opts))
		for _, o := range opts {
			if o.ResourceRecordName == nil || o.ResourceRecordType == nil || o.ResourceRecordValue == nil {
				continue
			}
			key := *o.ResourceRecordName
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, map[string]string{
				"name":  *o.ResourceRecordName,
				"type":  *o.ResourceRecordType,
				"value": *o.ResourceRecordValue,
			})
		}
		return out
	})

	// Build a small placeholder Record using only the first validation option.
	// In practice the validation set is 1-2 entries because we ask for one
	// apex + one wildcard, both validated against the same hostname. Picking
	// the first entry deterministically lets Pulumi materialise a single
	// concrete route53.Record. (For richer multi-SAN cases this should be
	// replaced with `dynamic.ResourceProvider` — out of scope here.)
	recName := validation.ApplyT(func(v []map[string]string) string {
		if len(v) == 0 {
			return ""
		}
		return v[0]["name"]
	}).(pulumi.StringOutput)
	recType := validation.ApplyT(func(v []map[string]string) string {
		if len(v) == 0 {
			return "CNAME"
		}
		return v[0]["type"]
	}).(pulumi.StringOutput)
	recValue := validation.ApplyT(func(v []map[string]string) string {
		if len(v) == 0 {
			return ""
		}
		return v[0]["value"]
	}).(pulumi.StringOutput)

	rec, err := route53.NewRecord(ctx, "cert-validation-"+name, &route53.RecordArgs{
		ZoneId:         zone.ZoneId,
		Name:           recName,
		Type:           recType,
		Ttl:            pulumi.Int(60),
		AllowOverwrite: pulumi.Bool(true),
		Records:        pulumi.StringArray{recValue},
	}, opts...)
	if err != nil {
		return nil, pulumi.StringOutput{}, err
	}

	if _, err := acm.NewCertificateValidation(ctx, "cert-validation-final-"+name, &acm.CertificateValidationArgs{
		CertificateArn:        cert.Arn,
		ValidationRecordFqdns: pulumi.StringArray{rec.Fqdn},
	}, opts...); err != nil {
		return nil, pulumi.StringOutput{}, err
	}

	return cert, cert.Arn, nil
}
