package data

import (
	"github.com/pulumi/pulumi-digitalocean/sdk/v4/go/digitalocean"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// Spaces is the trio of DO Spaces buckets the orchestrator + runtime
// use, plus the Spaces API key the orchestrator uses to sign uploads
// and the backup CronJob uses to push snapshots.
//
// Buckets:
//   - `backups`         — Postgres + SurrealDB dumps, 30-day expiration.
//   - `workspaces`      — per-user runtime snapshots, 7-day expiration
//                         (churn is high; we keep just enough to recover
//                         from a recent driver crash).
//   - `audit-exports`   — CSV/PDF audit downloads, 180-day expiration so
//                         the operator can still serve old links.
type Spaces struct {
	Endpoint  pulumi.StringOutput
	AccessKey pulumi.StringOutput // secret
	SecretKey pulumi.StringOutput // secret
	Buckets   map[string]pulumi.StringOutput
	Names     map[string]pulumi.StringOutput // bare bucket names (for s3:// URIs)
}

func provisionSpaces(ctx *pulumi.Context, in Inputs) (*Spaces, error) {
	cfg := in.Config
	region := cfg.SpacesRegion
	if region == "" {
		region = cfg.Region
	}

	type bucketSpec struct {
		logical    string
		expiration int  // days; 0 = no expiration rule
		cors       bool // attach a CORS configuration for browser GETs
	}
	specs := []bucketSpec{
		{"backups", 30, false},
		{"workspaces", 7, false},
		{"audit-exports", 180, true},
	}

	out := &Spaces{
		Endpoint: pulumi.String(spacesEndpoint(region)).ToStringOutput(),
		Buckets:  map[string]pulumi.StringOutput{},
		Names:    map[string]pulumi.StringOutput{},
	}

	for _, s := range specs {
		bucketName := cfg.ResourceName(s.logical)
		args := &digitalocean.SpacesBucketArgs{
			Name:       pulumi.String(bucketName),
			Region:     pulumi.String(region),
			Acl:        pulumi.String("private"),
			Versioning: &digitalocean.SpacesBucketVersioningArgs{Enabled: pulumi.Bool(true)},
		}
		if s.expiration > 0 {
			args.LifecycleRules = digitalocean.SpacesBucketLifecycleRuleArray{
				&digitalocean.SpacesBucketLifecycleRuleArgs{
					Id:      pulumi.String(s.logical + "-expire"),
					Enabled: pulumi.Bool(true),
					Expiration: &digitalocean.SpacesBucketLifecycleRuleExpirationArgs{
						Days: pulumi.Int(s.expiration),
					},
					AbortIncompleteMultipartUploadDays: pulumi.Int(1),
				},
			}
		}

		bucket, err := digitalocean.NewSpacesBucket(ctx, cfg.ResourceName("bucket-"+s.logical), args)
		if err != nil {
			return nil, err
		}

		if s.cors {
			origins := pulumi.StringArray{
				pulumi.String("https://app." + cfg.RootDomain),
				pulumi.String("https://" + cfg.RootDomain),
			}
			if cfg.VercelEnabled && cfg.VercelDomain != "" {
				origins = append(origins, pulumi.String("https://"+cfg.VercelDomain))
			}
			if _, err := digitalocean.NewSpacesBucketCorsConfiguration(ctx, cfg.ResourceName("bucket-"+s.logical+"-cors"), &digitalocean.SpacesBucketCorsConfigurationArgs{
				Bucket: bucket.Name,
				Region: pulumi.String(region),
				CorsRules: digitalocean.SpacesBucketCorsConfigurationCorsRuleArray{
					&digitalocean.SpacesBucketCorsConfigurationCorsRuleArgs{
						AllowedMethods: pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
						AllowedOrigins: origins,
						AllowedHeaders: pulumi.StringArray{pulumi.String("*")},
						ExposeHeaders:  pulumi.StringArray{pulumi.String("ETag"), pulumi.String("Content-Length")},
						MaxAgeSeconds:  pulumi.Int(3600),
					},
				},
			}); err != nil {
				return nil, err
			}
		}

		out.Buckets[s.logical] = bucket.BucketDomainName
		out.Names[s.logical] = bucket.Name
	}

	// Spaces API key — used by the orchestrator (S3-compatible client)
	// and the backup CronJob. Grants default to full-access in the DO
	// API when no Grants are listed, which is what we want for a
	// stack-scoped service key.
	key, err := digitalocean.NewSpacesKey(ctx, cfg.ResourceName("spaces-key"), &digitalocean.SpacesKeyArgs{
		Name: pulumi.String(cfg.ResourceName("spaces-key")),
	})
	if err != nil {
		return nil, err
	}
	out.AccessKey = key.AccessKey
	out.SecretKey = key.SecretKey

	return out, nil
}
