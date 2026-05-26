# Ironflyer Deploy Domains

This document describes the deployment/domain layer that makes Ironflyer feel
like a complete AppSec + deploy product for customer projects.

## Product Model

The competitor-grade flow is:

1. Build a preview from the customer project artifact.
2. Run deploy gates and approvals.
3. Promote to production.
4. Claim an instant managed subdomain.
5. Let the user connect an existing custom domain.
6. Optionally check and purchase a domain through a registrar.
7. Show DNS records, verification status, and TLS status in the publish flow.

The important boundary is that all of this attaches to the customer's project
and deploy rows. It must not expose Ironflyer internals, host paths, provider
credentials, or orchestration source code.

## What Exists Now

Backend package: `core/orchestrator/internal/operations/deploy`

Added domain lifecycle primitives:

- `DomainService`
- `DomainProvider`
- `Registrar`
- `MemoryDomainService`
- `PostgresDomainService`
- `StaticDomainProvider`
- `NoopRegistrar`
- `CloudflareRegistrar`

Persistence:

- Migration: `core/orchestrator/migrations/00042_deploy_domains.sql`
- Table: `deploy_domains`
- One project can have multiple domains.
- A primary domain is tracked per project.
- Domain state is separate from deploy state because domains can outlive one
  deploy.

GraphQL:

- `deployDomains(projectID: ID!)`
- `domainAvailability(domain: String!, registrar: String)`
- `reserveDeploySubdomain`
- `connectDeployDomain`
- `checkDeployDomain`
- `setPrimaryDeployDomain`
- `purchaseDeployDomain`

Studio UI:

- `PublishDialog` now shows a compact `Domains` block after the deploy is live.
- The UI auto-claims a managed Ironflyer subdomain when possible.
- The user can connect an existing domain and see DNS records.
- The user can check/purchase a domain when a real registrar adapter is
  configured.

## Default Safety

By default, domain purchase is disabled.

The default registrar is `manual`, implemented by `NoopRegistrar`. It reports
what is missing instead of charging money or sending registrar requests.

This is intentional. Domain purchase is a financial action and needs:

- Registrar account.
- Billing profile.
- Registrant contact data.
- Explicit operator configuration.
- Tenant/project secret release through the secrets broker.
- Server-side availability and price verification immediately before purchase.
- A hard maximum purchase price so margin cannot be destroyed by a premium
  quote or registrar drift.

## Configuration

The domain layer reads these environment variables during orchestrator wireup:

| Variable | Purpose | Default |
| --- | --- | --- |
| `IRONFLYER_MANAGED_DOMAIN_BASE` | Managed customer subdomain suffix | `ironflyer.app` |
| `IRONFLYER_EDGE_DNS_TARGET` | DNS target for custom domains | `edge.ironflyer.app` |
| `IRONFLYER_DOMAIN_PROVIDER` | Default domain provider adapter | `ironflyer` |
| `IRONFLYER_DOMAIN_REGISTRAR` | Default registrar adapter | `manual` |
| `CLOUDFLARE_ACCOUNT_ID` | Enables Cloudflare registrar adapter when secrets exist | empty |
| `IRONFLYER_DOMAIN_PURCHASE_ENABLED` | Enables real registrar purchase calls | `false` |
| `IRONFLYER_DOMAIN_MAX_PURCHASE_USD` | Hard cap for one domain purchase | `75` |
| `IRONFLYER_DOMAIN_PRICE_TOLERANCE_PCT` | Allowed server-side quote drift above UI quote | `10` |
| `IRONFLYER_DOMAIN_REQUIRE_CONTACT` | Requires registrant contact input before purchase | `false` |

Cloudflare purchase also requires a tenant/project secret named:

```text
CLOUDFLARE_API_TOKEN
```

The token is resolved through the existing secrets broker, not from UI state.

The browser's availability check is advisory. The orchestrator checks
availability and price again on the server before calling the registrar
purchase API. The purchase is blocked when:

- Purchase is not explicitly enabled.
- The registrar says the domain cannot be purchased.
- The registrar returns no positive price.
- The price exceeds `IRONFLYER_DOMAIN_MAX_PURCHASE_USD`.
- The price exceeds the expected UI quote plus tolerance.
- Contact details are required by policy but absent.

## Domain States

Kinds:

- `managed_subdomain`
- `connected_domain`
- `registered_domain`

Statuses:

- `pending_dns`
- `verifying`
- `live`
- `failed`
- `removed`

Certificate statuses:

- `pending`
- `active`
- `failed`

The static provider marks managed subdomains live immediately. Custom domains
return DNS records and stay pending/verifying until provider verification
confirms ownership and TLS.

## DNS Contract

For subdomains, Ironflyer emits:

- `CNAME hostname -> edge target`
- `TXT _ironflyer.hostname -> ironflyer-verify=...`

For apex domains, Ironflyer emits:

- `ALIAS @ -> edge target`
- `TXT _ironflyer.hostname -> ironflyer-verify=...`

Real provider adapters can replace this with provider-specific records such as
Cloudflare Pages custom hostnames, Vercel domain verification, Route53 alias
records, or Fly.io certificates.

## Why This Matches Competitors

Vercel, Netlify, Render, Fly, Replit, and similar products separate the flow
into the same concepts:

- Build/promote a deploy artifact.
- Give the user an instant platform subdomain.
- Let the user connect a domain by adding DNS records.
- Verify ownership with TXT/CNAME records.
- Issue TLS after DNS verification.
- Offer optional domain purchase through a registrar partner or registrar API.

Ironflyer now has those product surfaces in the core architecture. The first
provider is intentionally conservative and replaceable.

## Provider Roadmap

High priority:

- Vercel domain adapter using Vercel project/domain APIs.
- Cloudflare Pages/Workers custom hostname adapter.
- DNS verification that actually resolves TXT/CNAME records.
- ACME/TLS status polling from the deploy provider.
- Audit events for domain connect, verify, primary switch, and purchase.
- Tenant policy: who may purchase domains and max annual spend.

Medium priority:

- Route53 adapter for teams that own AWS DNS.
- Fly.io certificates/custom domains.
- Namecheap/GoDaddy adapters only if terms and API stability justify them.
- Domain renewal/expiry webhook ingestion.
- Domain transfer-in flow.
- WHOIS/RDAP enrichment for risk hints.

Do not build first:

- A full DNS hosting product.
- Manual zone editor.
- Domain brokerage.
- Bulk domain portfolio management.

## Security Notes

The domain layer must keep these rules:

- Never scan or expose Ironflyer source when operating on customer deploys.
- Never leak registrar tokens to the browser.
- Never buy a domain unless the configured registrar confirms availability and
  the expected price matches policy.
- Keep TXT verification records stable so users can retry checks safely.
- Treat domain ownership as tenant-scoped and project-scoped.
- Keep purchase disabled by default in local/dev environments.

## UX Principle

The dashboard stays clean. Domain intelligence belongs in the publish flow and
deploy detail page:

- Live URL.
- Primary domain.
- Pending DNS records.
- Verification/TLS status.
- A compact action to connect or buy.

No noisy domain tables on the main dashboard unless there is an actionable
problem.
