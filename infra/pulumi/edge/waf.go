package edge

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/wafv2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"ironflyer/infra/pulumi/compute"
)

// NewWAF creates a WAFv2 web ACL scoped to CLOUDFRONT (mandatory us-east-1).
// The ACL stacks three managed rule sets, a per-IP rate-limit, and an
// optional allow-list rule. Logs go to a CloudWatch log group; CloudFront's
// access logs go to S3 separately (in cdn.go).
func NewWAF(ctx *pulumi.Context, cfg *compute.Config, usEast1 *aws.Provider) (*wafv2.WebAcl, error) {
	opts := pulumi.Provider(usEast1)

	rules := wafv2.WebAclRuleArray{
		managedRule("AWS-Common", "AWSManagedRulesCommonRuleSet", 10),
		managedRule("AWS-KnownBadInputs", "AWSManagedRulesKnownBadInputsRuleSet", 20),
		managedRule("AWS-SQLi", "AWSManagedRulesSQLiRuleSet", 30),
		&wafv2.WebAclRuleArgs{
			Name:     pulumi.String("rate-limit"),
			Priority: pulumi.Int(40),
			Action: &wafv2.WebAclRuleActionArgs{
				Block: &wafv2.WebAclRuleActionBlockArgs{
					CustomResponse: &wafv2.WebAclRuleActionBlockCustomResponseArgs{
						ResponseCode: pulumi.Int(429),
					},
				},
			},
			Statement: &wafv2.WebAclRuleStatementArgs{
				RateBasedStatement: &wafv2.WebAclRuleStatementRateBasedStatementArgs{
					Limit:            pulumi.Int(1000),
					AggregateKeyType: pulumi.String("IP"),
				},
			},
			VisibilityConfig: &wafv2.WebAclRuleVisibilityConfigArgs{
				CloudwatchMetricsEnabled: pulumi.Bool(true),
				MetricName:               pulumi.String("ironflyer-rate-limit"),
				SampledRequestsEnabled:   pulumi.Bool(true),
			},
		},
	}

	// Optional allow-list — only present when the operator configured IPs.
	if len(cfg.AllowlistedIPs) > 0 {
		ips := make(pulumi.StringArray, 0, len(cfg.AllowlistedIPs))
		for _, ip := range cfg.AllowlistedIPs {
			ips = append(ips, pulumi.String(ip))
		}
		ipset, err := wafv2.NewIpSet(ctx, "waf-allowlist", &wafv2.IpSetArgs{
			Description:      pulumi.String("Ironflyer monitoring + ops IP allow-list"),
			Scope:            pulumi.String("CLOUDFRONT"),
			IpAddressVersion: pulumi.String("IPV4"),
			Addresses:        ips,
			Tags:             cfg.Tags(),
		}, opts)
		if err != nil {
			return nil, err
		}
		rules = append(rules, &wafv2.WebAclRuleArgs{
			Name:     pulumi.String("allowlist"),
			Priority: pulumi.Int(1),
			Action: &wafv2.WebAclRuleActionArgs{
				Allow: &wafv2.WebAclRuleActionAllowArgs{},
			},
			Statement: &wafv2.WebAclRuleStatementArgs{
				IpSetReferenceStatement: &wafv2.WebAclRuleStatementIpSetReferenceStatementArgs{
					Arn: ipset.Arn,
				},
			},
			VisibilityConfig: &wafv2.WebAclRuleVisibilityConfigArgs{
				CloudwatchMetricsEnabled: pulumi.Bool(true),
				MetricName:               pulumi.String("ironflyer-allowlist"),
				SampledRequestsEnabled:   pulumi.Bool(true),
			},
		})
	}

	acl, err := wafv2.NewWebAcl(ctx, "ironflyer-acl", &wafv2.WebAclArgs{
		Description: pulumi.String("Ironflyer " + cfg.Stack + " edge ACL"),
		Scope:       pulumi.String("CLOUDFRONT"),
		DefaultAction: &wafv2.WebAclDefaultActionArgs{
			Allow: &wafv2.WebAclDefaultActionAllowArgs{},
		},
		Rules: rules,
		VisibilityConfig: &wafv2.WebAclVisibilityConfigArgs{
			CloudwatchMetricsEnabled: pulumi.Bool(true),
			MetricName:               pulumi.String("ironflyer-edge-acl"),
			SampledRequestsEnabled:   pulumi.Bool(true),
		},
		Tags: cfg.Tags(),
	}, opts)
	if err != nil {
		return nil, err
	}

	// CloudWatch log group for WAF logs. Name MUST start with `aws-waf-logs-`.
	logGroup, err := cloudwatch.NewLogGroup(ctx, "waf-logs", &cloudwatch.LogGroupArgs{
		Name:            pulumi.String("aws-waf-logs-ironflyer-" + cfg.Stack),
		RetentionInDays: pulumi.Int(30),
		Tags:            cfg.Tags(),
	}, opts)
	if err != nil {
		return nil, err
	}
	if _, err := wafv2.NewWebAclLoggingConfiguration(ctx, "waf-logs-config", &wafv2.WebAclLoggingConfigurationArgs{
		LogDestinationConfigs: pulumi.StringArray{logGroup.Arn},
		ResourceArn:           acl.Arn,
	}, opts); err != nil {
		return nil, err
	}

	return acl, nil
}

func managedRule(name, ruleName string, priority int) *wafv2.WebAclRuleArgs {
	return &wafv2.WebAclRuleArgs{
		Name:     pulumi.String(name),
		Priority: pulumi.Int(priority),
		OverrideAction: &wafv2.WebAclRuleOverrideActionArgs{
			None: &wafv2.WebAclRuleOverrideActionNoneArgs{},
		},
		Statement: &wafv2.WebAclRuleStatementArgs{
			ManagedRuleGroupStatement: &wafv2.WebAclRuleStatementManagedRuleGroupStatementArgs{
				VendorName: pulumi.String("AWS"),
				Name:       pulumi.String(ruleName),
			},
		},
		VisibilityConfig: &wafv2.WebAclRuleVisibilityConfigArgs{
			CloudwatchMetricsEnabled: pulumi.Bool(true),
			MetricName:               pulumi.String("ironflyer-" + name),
			SampledRequestsEnabled:   pulumi.Bool(true),
		},
	}
}
