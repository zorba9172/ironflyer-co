package edge

import (
	"github.com/pulumi/pulumi-cloudflare/sdk/v5/go/cloudflare"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// newCloudflare wires the public DNS records, zone hardening, and WAF
// rules in Cloudflare. Cloudflare is the authoritative DNS + edge security
// layer for every Ironflyer stack — DigitalOcean's own DNS service is
// intentionally unused so the WAF + proxying are uniform across both DO
// and AWS stacks (the AWS stack uses CloudFront + WAFv2 + Route53; here
// Cloudflare covers the same three responsibilities in one product).
//
// Records:
//   - api.<root>     A      → ingress LB IP, proxied (Cloudflare WAF in front).
//   - runtime.<root> A      → ingress LB IP, proxied.
//   - app.<root>     CNAME  → cname.vercel-dns.com, NOT proxied
//                              (Vercel issues + serves its own TLS).
//   - docs.<root>    CNAME  → cname.vercel-dns.com, NOT proxied.
//
// Zone settings tighten the defaults: TLS 1.2 minimum, Always Use HTTPS,
// HSTS one year incl. subdomains + preload, Always Online for cached
// content during origin outages, security level High.
//
// WAF: a single custom Ruleset under phase `http_request_firewall_custom`
// covers (a) explicit GET-only allowlist for the orchestrator API
// (subscription POSTs still allowed) — pairs with Cloudflare's managed
// bot-fight at the zone level — and (b) a rate-limit ruleset for
// `/graphql` mutations.
//
// Page rules: docs.* gets aggressive 1d cache; api.* explicitly bypasses
// cache (GraphQL is dynamic).
func newCloudflare(ctx *pulumi.Context, in Inputs, lbIP pulumi.StringOutput) error {
	cfg := in.Config
	root := rootDomain(cfg)
	if root == "" {
		// Without a root domain we cannot create zone-scoped resources;
		// the Cloudflare resources are optional for dev stacks that
		// don't own a public zone.
		ctx.Log.Warn("edge.cloudflare: ironflyer:rootDomain is empty; skipping Cloudflare provisioning", nil)
		return nil
	}

	cfConfig := config.New(ctx, "cloudflare")
	apiToken := cfConfig.RequireSecret("apiToken")

	prov, err := cloudflare.NewProvider(ctx, "cloudflare-"+cfg.Stack, &cloudflare.ProviderArgs{
		ApiToken: apiToken,
	})
	if err != nil {
		return err
	}
	provOpt := pulumi.Provider(prov)

	// Resolve the zone ID by name. The zone itself is created out-of-band
	// (Cloudflare requires a one-time NS hand-off to register name
	// servers) — we don't manage `cloudflare.Zone` in Pulumi because the
	// hand-off can't be rerun safely on every `pulumi up`.
	zone, err := cloudflare.LookupZoneOutput(ctx, cloudflare.LookupZoneOutputArgs{
		Name: pulumi.String(root),
	}, provOpt).ID().ToStringOutput(), error(nil)
	if err != nil {
		return err
	}
	_ = zone

	zoneID := cloudflare.LookupZoneOutput(ctx, cloudflare.LookupZoneOutputArgs{
		Name: pulumi.String(root),
	}, provOpt).ApplyT(func(z cloudflare.LookupZoneResult) string {
		return z.Id
	}).(pulumi.StringOutput)

	// --- DNS records ----------------------------------------------------
	if apiHost := apiHostname(cfg); apiHost != "" {
		if _, err := cloudflare.NewRecord(ctx, "api-a", &cloudflare.RecordArgs{
			ZoneId:  zoneID,
			Name:    pulumi.String(apiHost),
			Type:    pulumi.String("A"),
			Value:   lbIP,
			Ttl:     pulumi.Int(1), // 1 = auto/managed when proxied
			Proxied: pulumi.Bool(true),
			Comment: pulumi.String("ironflyer orchestrator API (" + cfg.Stack + ")"),
		}, provOpt); err != nil {
			return err
		}
	}

	if runtimeHost := runtimeHostname(cfg); runtimeHost != "" {
		if _, err := cloudflare.NewRecord(ctx, "runtime-a", &cloudflare.RecordArgs{
			ZoneId:  zoneID,
			Name:    pulumi.String(runtimeHost),
			Type:    pulumi.String("A"),
			Value:   lbIP,
			Ttl:     pulumi.Int(1),
			Proxied: pulumi.Bool(true),
			Comment: pulumi.String("ironflyer workspace runtime (" + cfg.Stack + ")"),
		}, provOpt); err != nil {
			return err
		}
	}

	// errors.<root> — self-hosted GlitchTip (Sentry-wire-compatible).
	// Backed by the same DOKS ingress LB as api.<root>; the chart's
	// templates/glitchtip.yaml Ingress matches on this hostname.
	// Proxied through Cloudflare so the GlitchTip envelope endpoint
	// inherits the same WAF + DDoS shield as the rest of the stack.
	if errorsHost := errorsHostname(cfg); errorsHost != "" {
		if _, err := cloudflare.NewRecord(ctx, "errors-a", &cloudflare.RecordArgs{
			ZoneId:  zoneID,
			Name:    pulumi.String(errorsHost),
			Type:    pulumi.String("A"),
			Value:   lbIP,
			Ttl:     pulumi.Int(1),
			Proxied: pulumi.Bool(true),
			Comment: pulumi.String("ironflyer GlitchTip error tracker (" + cfg.Stack + ")"),
		}, provOpt); err != nil {
			return err
		}
	}

	if appHost := appHostname(cfg); appHost != "" && cfg.VercelEnabled {
		if _, err := cloudflare.NewRecord(ctx, "app-cname", &cloudflare.RecordArgs{
			ZoneId:  zoneID,
			Name:    pulumi.String(appHost),
			Type:    pulumi.String("CNAME"),
			Value:   pulumi.String("cname.vercel-dns.com"),
			Ttl:     pulumi.Int(300),
			Proxied: pulumi.Bool(false), // Vercel terminates TLS
			Comment: pulumi.String("ironflyer dashboard → Vercel (" + cfg.Stack + ")"),
		}, provOpt); err != nil {
			return err
		}
	}

	if docsHost := docsHostname(cfg); docsHost != "" {
		if _, err := cloudflare.NewRecord(ctx, "docs-cname", &cloudflare.RecordArgs{
			ZoneId:  zoneID,
			Name:    pulumi.String(docsHost),
			Type:    pulumi.String("CNAME"),
			Value:   pulumi.String("cname.vercel-dns.com"),
			Ttl:     pulumi.Int(300),
			Proxied: pulumi.Bool(false),
			Comment: pulumi.String("ironflyer docs (" + cfg.Stack + ")"),
		}, provOpt); err != nil {
			return err
		}
	}

	// --- Zone settings --------------------------------------------------
	if _, err := cloudflare.NewZoneSettingsOverride(ctx, "ironflyer-zone-settings", &cloudflare.ZoneSettingsOverrideArgs{
		ZoneId: zoneID,
		Settings: &cloudflare.ZoneSettingsOverrideSettingsArgs{
			MinTlsVersion:    pulumi.String("1.2"),
			AlwaysUseHttps:   pulumi.String("on"),
			AutomaticHttpsRewrites: pulumi.String("on"),
			AlwaysOnline:     pulumi.String("on"),
			SecurityLevel:    pulumi.String("high"),
			BrowserCheck:     pulumi.String("on"),
			OpportunisticEncryption: pulumi.String("on"),
			Tls13:            pulumi.String("on"),
			Http3:            pulumi.String("on"),
			ZeroRtt:          pulumi.String("on"),
			SecurityHeader: &cloudflare.ZoneSettingsOverrideSettingsSecurityHeaderArgs{
				Enabled:           pulumi.Bool(true),
				IncludeSubdomains: pulumi.Bool(true),
				MaxAge:            pulumi.Int(31536000),
				Preload:           pulumi.Bool(true),
				Nosniff:           pulumi.Bool(true),
			},
		},
	}, provOpt); err != nil {
		return err
	}

	// --- WAF custom ruleset --------------------------------------------
	// Cloudflare's free + Pro plans expose the `http_request_firewall_custom`
	// phase; the rules below stack on top of any managed rulesets the
	// operator has enabled in the dashboard.
	if _, err := cloudflare.NewRuleset(ctx, "ironflyer-waf", &cloudflare.RulesetArgs{
		ZoneId:      zoneID,
		Name:        pulumi.String("ironflyer-edge-waf"),
		Description: pulumi.String("Ironflyer custom edge WAF rules (" + cfg.Stack + ")"),
		Kind:        pulumi.String("zone"),
		Phase:       pulumi.String("http_request_firewall_custom"),
		Rules: cloudflare.RulesetRuleArray{
			&cloudflare.RulesetRuleArgs{
				Action:      pulumi.String("block"),
				Description: pulumi.String("Block known bad bots flagged by Cloudflare bot management"),
				Expression:  pulumi.String("(cf.client.bot) and not (cf.verified_bot_category in {\"Search Engine\" \"Monitoring & Analytics\"})"),
				Enabled:     pulumi.Bool(true),
			},
			&cloudflare.RulesetRuleArgs{
				Action:      pulumi.String("block"),
				Description: pulumi.String("Reject PUT/DELETE on api.* — GraphQL is POST/GET only"),
				Expression:  pulumi.Sprintf("(http.host eq \"%s\" and http.request.method in {\"PUT\" \"DELETE\" \"PATCH\"})", apiHostname(cfg)),
				Enabled:     pulumi.Bool(true),
			},
		},
	}, provOpt); err != nil {
		return err
	}

	// Rate-limit `/graphql` POSTs at the edge (defence-in-depth — the
	// orchestrator's own middleware enforces a stricter per-token limit).
	if _, err := cloudflare.NewRuleset(ctx, "ironflyer-ratelimit", &cloudflare.RulesetArgs{
		ZoneId:      zoneID,
		Name:        pulumi.String("ironflyer-edge-ratelimit"),
		Description: pulumi.String("Rate-limit POST /graphql to 600 req/min/IP (" + cfg.Stack + ")"),
		Kind:        pulumi.String("zone"),
		Phase:       pulumi.String("http_ratelimit"),
		Rules: cloudflare.RulesetRuleArray{
			&cloudflare.RulesetRuleArgs{
				Action:      pulumi.String("block"),
				Description: pulumi.String("Per-IP rate-limit on POST /graphql"),
				Expression:  pulumi.Sprintf("(http.host eq \"%s\" and http.request.method eq \"POST\" and starts_with(http.request.uri.path, \"/graphql\"))", apiHostname(cfg)),
				Enabled:     pulumi.Bool(true),
				Ratelimit: &cloudflare.RulesetRuleRatelimitArgs{
					Characteristics: pulumi.StringArray{
						pulumi.String("cf.colo.id"),
						pulumi.String("ip.src"),
					},
					Period:            pulumi.Int(60),
					RequestsPerPeriod: pulumi.Int(600),
					MitigationTimeout: pulumi.Int(60),
				},
			},
		},
	}, provOpt); err != nil {
		return err
	}

	// --- Page rules (cache control) ------------------------------------
	if docsHost := docsHostname(cfg); docsHost != "" {
		if _, err := cloudflare.NewPageRule(ctx, "docs-cache", &cloudflare.PageRuleArgs{
			ZoneId:   zoneID,
			Target:   pulumi.String(docsHost + "/*"),
			Priority: pulumi.Int(2),
			Status:   pulumi.String("active"),
			Actions: &cloudflare.PageRuleActionsArgs{
				CacheLevel:       pulumi.String("cache_everything"),
				EdgeCacheTtl:     pulumi.Int(86400),
				BrowserCacheTtl:  pulumi.String("3600"),
			},
		}, provOpt); err != nil {
			return err
		}
	}

	if apiHost := apiHostname(cfg); apiHost != "" {
		if _, err := cloudflare.NewPageRule(ctx, "api-no-cache", &cloudflare.PageRuleArgs{
			ZoneId:   zoneID,
			Target:   pulumi.String(apiHost + "/*"),
			Priority: pulumi.Int(1),
			Status:   pulumi.String("active"),
			Actions: &cloudflare.PageRuleActionsArgs{
				CacheLevel:      pulumi.String("bypass"),
				DisablePerformance: pulumi.Bool(true),
			},
		}, provOpt); err != nil {
			return err
		}
	}

	return nil
}
