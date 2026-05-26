# `core/orchestrator/internal/` — domain layout

The orchestrator's internal packages are physically grouped into five
business domains plus a cross-cutting shared-utility layer. The
top-level layout is:

```
internal/
├── business/    revenue, cost, margin, financial truth
├── ai/          model orchestration, prompts, retrieval, memory, scoring
├── operations/  execution lifecycle, gates, deploy, runtime, observability
├── customer/    signup, auth, notification, lifecycle
├── suppliers/   third-party adapter modules that have no first-class domain
└── pkg/         cross-cutting shared utilities (env, httputil, httpclient)
```

## Why `pkg/` sits at the top

`pkg/env`, `pkg/httputil`, and `pkg/httpclient` are pure helpers that
every domain may import. Treating them as a sixth domain would
overstate them; nesting them inside one domain would force the others
to reach across domain boundaries to use a generic helper. Keeping
them flat at the root makes the import direction explicit: domains
depend on `pkg`, never the other way round.

## Why `suppliers/` is intentionally lean

Most vendor adapters live as files *inside* the domain that owns the
contract:

- Anthropic / OpenAI / Gemini live under `ai/providers` because the
  contract is "model call" — owned by AI.
- Stripe lives under `business/budget` because the contract is
  "charge a wallet" — owned by Business.
- Vercel lives under `operations/deploy` because the contract is
  "ship an artifact" — owned by Operations.
- S3 lives under `operations/storage` because the contract is
  "store an object" — owned by Operations.
- Resend / SendGrid live under `customer/notify` because the contract
  is "deliver a customer email" — owned by Customer.

Moving those out would split the domain that owns the contract.
`suppliers/` is reserved for adapter packages that exist purely as
third-party integration surface with no first-class domain home —
e.g. on the runtime side, `suppliers/mobile` for Appetize / EAS /
BrowserStack adapters.

## Where each domain's full package list lives

See [`docs/ARCHITECTURE_DOMAIN_MODULES.md`](../../../docs/ARCHITECTURE_DOMAIN_MODULES.md)
for the canonical one-line-per-package map. That document is updated
in lockstep with this tree.
