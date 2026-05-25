package edge

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudfront"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/s3"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/wafv2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"ironflyer/infra/pulumi/compute"
)

// NewCDN creates the CloudFront distribution that fronts the platform.
//
// Two origins: the orchestrator ALB (API + webhook + REST allow-list) and
// the web SPA S3 bucket (the data Pulumi stack owns that bucket; pass its
// name via `infra:webSpaBucketName` or via a StackReference contract).
//
// Path-based dispatch:
//   - Default behavior:     S3 (web SPA), long cache, immutable assets.
//   - `/graphql*` and the explicit REST allow-list go to the ALB with all
//     headers/cookies forwarded and TTL=0.
//   - `/_next/static/*`, `/static/*`, `/assets/*` get a 1-year immutable
//     cache.
//
// HTTP→HTTPS redirect is enforced. Minimum protocol is TLS 1.2_2021.
// HTTP/2 + HTTP/3 are enabled. WAFv2 web ACL attaches at the edge.
func NewCDN(ctx *pulumi.Context, cfg *compute.Config, certs *Certs, acl *wafv2.WebAcl) error {
	// CloudFront access logs — bucket lives in the same account, encrypted.
	logsBucket, err := s3.NewBucketV2(ctx, "cf-logs", &s3.BucketV2Args{
		Bucket:       pulumi.String("cf-logs.ironflyer." + cfg.Stack),
		ForceDestroy: pulumi.Bool(false),
		Tags:         cfg.TagsWith(map[string]string{"Purpose": "cloudfront-logs"}),
	})
	if err != nil {
		return err
	}
	if _, err := s3.NewBucketServerSideEncryptionConfigurationV2(ctx, "cf-logs-sse", &s3.BucketServerSideEncryptionConfigurationV2Args{
		Bucket: logsBucket.ID(),
		Rules: s3.BucketServerSideEncryptionConfigurationV2RuleArray{&s3.BucketServerSideEncryptionConfigurationV2RuleArgs{
			ApplyServerSideEncryptionByDefault: &s3.BucketServerSideEncryptionConfigurationV2RuleApplyServerSideEncryptionByDefaultArgs{
				SseAlgorithm: pulumi.String("AES256"),
			},
		}},
	}); err != nil {
		return err
	}
	if _, err := s3.NewBucketLifecycleConfigurationV2(ctx, "cf-logs-lifecycle", &s3.BucketLifecycleConfigurationV2Args{
		Bucket: logsBucket.ID(),
		Rules: s3.BucketLifecycleConfigurationV2RuleArray{&s3.BucketLifecycleConfigurationV2RuleArgs{
			Id:     pulumi.String("expire-30d"),
			Status: pulumi.String("Enabled"),
			Expiration: &s3.BucketLifecycleConfigurationV2RuleExpirationArgs{
				Days: pulumi.Int(30),
			},
		}},
	}); err != nil {
		return err
	}
	if _, err := s3.NewBucketOwnershipControls(ctx, "cf-logs-ownership", &s3.BucketOwnershipControlsArgs{
		Bucket: logsBucket.ID(),
		Rule: &s3.BucketOwnershipControlsRuleArgs{
			ObjectOwnership: pulumi.String("BucketOwnerPreferred"),
		},
	}); err != nil {
		return err
	}

	// Web SPA bucket — owned by the data stack. If config gives us the
	// name we use it; otherwise we fall back to the well-known convention.
	webBucketName := cfg.WebSpaBucket
	if webBucketName == "" {
		webBucketName = "ironflyer-" + cfg.Stack + "-web"
	}

	// Origin Access Control so the S3 bucket can stay private.
	oac, err := cloudfront.NewOriginAccessControl(ctx, "web-oac", &cloudfront.OriginAccessControlArgs{
		Name:                          pulumi.String("ironflyer-" + cfg.Stack + "-web-oac"),
		Description:                   pulumi.String("Web SPA origin access control"),
		OriginAccessControlOriginType: pulumi.String("s3"),
		SigningBehavior:               pulumi.String("always"),
		SigningProtocol:               pulumi.String("sigv4"),
	})
	if err != nil {
		return err
	}

	// The orchestrator ALB DNS name. We can't know it concretely at plan
	// time (the AWS Load Balancer Controller creates it from an Ingress).
	// We register the expected DNS name (controlled by external-dns).
	apiHost := cfg.APIHostname()

	originReqAll, err := cloudfront.NewOriginRequestPolicy(ctx, "api-origin-req", &cloudfront.OriginRequestPolicyArgs{
		Name: pulumi.String("ironflyer-" + cfg.Stack + "-api-origin-req"),
		CookiesConfig: &cloudfront.OriginRequestPolicyCookiesConfigArgs{
			CookieBehavior: pulumi.String("all"),
		},
		HeadersConfig: &cloudfront.OriginRequestPolicyHeadersConfigArgs{
			HeaderBehavior: pulumi.String("allViewer"),
		},
		QueryStringsConfig: &cloudfront.OriginRequestPolicyQueryStringsConfigArgs{
			QueryStringBehavior: pulumi.String("all"),
		},
	})
	if err != nil {
		return err
	}

	// Pulumi managed cache policies (well-known IDs) — pull them via
	// data sources so we get the right ARN/ID per region. Using the
	// well-known UUIDs keeps the program declarative.
	const (
		cachingDisabledID  = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad" // CachingDisabled
		cachingOptimizedID = "658327ea-f89d-4fab-a63d-7e88639e58f6" // CachingOptimized
	)

	dist, err := cloudfront.NewDistribution(ctx, "ironflyer-cdn", &cloudfront.DistributionArgs{
		Enabled:           pulumi.Bool(true),
		IsIpv6Enabled:     pulumi.Bool(true),
		HttpVersion:       pulumi.String("http2and3"),
		PriceClass:        pulumi.String("PriceClass_100"),
		WebAclId:          acl.Arn,
		DefaultRootObject: pulumi.String("index.html"),
		Aliases: pulumi.StringArray{
			pulumi.String(cfg.WebHostname()),
		},
		Origins: cloudfront.DistributionOriginArray{
			&cloudfront.DistributionOriginArgs{
				OriginId:              pulumi.String("s3-web"),
				DomainName:            pulumi.String(webBucketName + ".s3.amazonaws.com"),
				OriginAccessControlId: oac.ID(),
			},
			&cloudfront.DistributionOriginArgs{
				OriginId:   pulumi.String("alb-api"),
				DomainName: pulumi.String(apiHost),
				CustomOriginConfig: &cloudfront.DistributionOriginCustomOriginConfigArgs{
					HttpPort:             pulumi.Int(80),
					HttpsPort:            pulumi.Int(443),
					OriginProtocolPolicy: pulumi.String("https-only"),
					OriginSslProtocols:   pulumi.StringArray{pulumi.String("TLSv1.2")},
				},
			},
		},
		DefaultCacheBehavior: &cloudfront.DistributionDefaultCacheBehaviorArgs{
			TargetOriginId:       pulumi.String("s3-web"),
			ViewerProtocolPolicy: pulumi.String("redirect-to-https"),
			AllowedMethods:       pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD"), pulumi.String("OPTIONS")},
			CachedMethods:        pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
			Compress:             pulumi.Bool(true),
			CachePolicyId:        pulumi.String(cachingOptimizedID),
		},
		OrderedCacheBehaviors: cloudfront.DistributionOrderedCacheBehaviorArray{
			apiBehavior("/graphql*", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/webhooks/*", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/budget/webhook", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/healthz", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/livez", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/readyz", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/version", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/metrics", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/openapi.yaml", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/openapi.json", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/docs", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/r/*", "alb-api", originReqAll.ID(), cachingDisabledID),
			apiBehavior("/shared/*", "alb-api", originReqAll.ID(), cachingDisabledID),
			staticBehavior("/static/*", cachingOptimizedID),
			staticBehavior("/_next/static/*", cachingOptimizedID),
			staticBehavior("/assets/*", cachingOptimizedID),
		},
		ViewerCertificate: &cloudfront.DistributionViewerCertificateArgs{
			AcmCertificateArn:      certs.UsEast1Arn,
			SslSupportMethod:       pulumi.String("sni-only"),
			MinimumProtocolVersion: pulumi.String("TLSv1.2_2021"),
		},
		Restrictions: &cloudfront.DistributionRestrictionsArgs{
			GeoRestriction: &cloudfront.DistributionRestrictionsGeoRestrictionArgs{
				RestrictionType: pulumi.String("none"),
			},
		},
		LoggingConfig: &cloudfront.DistributionLoggingConfigArgs{
			Bucket:         logsBucket.BucketDomainName,
			IncludeCookies: pulumi.Bool(false),
			Prefix:         pulumi.String("cloudfront/"),
		},
		Tags: cfg.Tags(),
	})
	if err != nil {
		return err
	}

	ctx.Export("cloudFrontDomainName", dist.DomainName)
	ctx.Export("cloudFrontDistributionId", dist.ID())
	ctx.Export("cloudFrontLogsBucket", logsBucket.Bucket)
	return nil
}

func apiBehavior(pattern, originID string, originReqPolicyID pulumi.IDOutput, cachePolicyID string) *cloudfront.DistributionOrderedCacheBehaviorArgs {
	return &cloudfront.DistributionOrderedCacheBehaviorArgs{
		PathPattern:           pulumi.String(pattern),
		TargetOriginId:        pulumi.String(originID),
		ViewerProtocolPolicy:  pulumi.String("redirect-to-https"),
		AllowedMethods:        pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD"), pulumi.String("OPTIONS"), pulumi.String("PUT"), pulumi.String("POST"), pulumi.String("PATCH"), pulumi.String("DELETE")},
		CachedMethods:         pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
		Compress:              pulumi.Bool(true),
		CachePolicyId:         pulumi.String(cachePolicyID),
		OriginRequestPolicyId: originReqPolicyID.ToStringOutput(),
	}
}

func staticBehavior(pattern, cachePolicyID string) *cloudfront.DistributionOrderedCacheBehaviorArgs {
	return &cloudfront.DistributionOrderedCacheBehaviorArgs{
		PathPattern:          pulumi.String(pattern),
		TargetOriginId:       pulumi.String("s3-web"),
		ViewerProtocolPolicy: pulumi.String("redirect-to-https"),
		AllowedMethods:       pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
		CachedMethods:        pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
		Compress:             pulumi.Bool(true),
		CachePolicyId:        pulumi.String(cachePolicyID),
	}
}
