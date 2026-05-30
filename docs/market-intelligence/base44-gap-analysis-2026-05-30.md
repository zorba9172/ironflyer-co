# Base44 Mapping and Gap Analysis (2026-05-30)

## 1) What was mapped on Base44

### Core product pages
- Homepage: https://base44.com/
- Features: https://base44.com/features
- Integrations: https://base44.com/integrations
- Templates: https://base44.com/templates
- Use Cases: https://base44.com/use-cases
- Pricing: https://base44.com/pricing
- Enterprise: https://base44.com/enterprise
- Backend platform: https://base44.com/backend
- Superagents: https://base44.com/superagents
- AI App Builder: https://base44.com/ai-app-builder
- AI Website Builder: https://base44.com/ai-website-builder
- Changelog: https://base44.com/changelog
- Docs hub: https://docs.base44.com/

### High-signal market-intelligence pages
- Marketing and Sales category: https://base44.com/use-cases/categories/marketing-and-sales
- Business Intelligence and Analytics category: https://base44.com/use-cases/categories/business-intelligence-and-analytics
- CRM category: https://base44.com/use-cases/categories/crm
- Marketing templates: https://base44.com/templates/categories/marketing-sales-automation
- BI templates: https://base44.com/templates/categories/business-intelligence-analytics
- HubSpot connector: https://base44.com/integrations/connectors/hubspot
- Salesforce connector: https://base44.com/integrations/connectors/salesforce
- LinkedIn connector: https://base44.com/integrations/connectors/linkedin
- Google Sheets connector: https://base44.com/integrations/connectors/google-sheets

### Taxonomy density observed
- Use-case categories: 13 visible categories.
- Marketing and Sales leaf use cases: 6.
- BI and Analytics leaf use cases: 4.
- CRM leaf use cases: 6.
- Template categories include explicit category counts and clone-count social proof.
- Changelog shows sustained monthly release cadence with many feature-level entries.

## 2) What exists locally (current baseline)

### Marketing surface in this repo
- Routes currently shipped in marketing client:
  - [ironflyer/clients/marketing/src/routes.tsx](ironflyer/clients/marketing/src/routes.tsx)
  - Current routes: home, product, studio, pricing, manifesto.
- Product narrative page:
  - [ironflyer/clients/marketing/src/pages/Product.tsx](ironflyer/clients/marketing/src/pages/Product.tsx)
- Pricing narrative page:
  - [ironflyer/clients/marketing/src/pages/Pricing.tsx](ironflyer/clients/marketing/src/pages/Pricing.tsx)
- Manifesto page:
  - [ironflyer/clients/marketing/src/pages/Manifesto.tsx](ironflyer/clients/marketing/src/pages/Manifesto.tsx)

### Additional public marketing-like surfaces in web client
- Base44-like public page component with modes such as templates/solutions/pricing/resources/enterprise:
  - [ironflyer/clients/web/src/components/marketing/Base44PublicPage.tsx](ironflyer/clients/web/src/components/marketing/Base44PublicPage.tsx)
- Public changelog page:
  - [ironflyer/clients/web/app/changelog/page.tsx](ironflyer/clients/web/app/changelog/page.tsx)

## 3) Gap map: what Base44 has that is not yet strongly present locally

### A. SEO surface area and intent capture
- Base44 has a wide intent net: many category pages + leaf use-case pages + connector pages + blog clusters.
- Local marketing client is compact and narrative-heavy, with limited intent landing pages.
- Impact: weaker long-tail discovery for terms like "marketing analytics app", "lead nurturing tool", "sales dashboard", "HubSpot automation app".

### B. Market-intelligence specific taxonomy
- Base44 explicitly packages GTM intelligence by category:
  - Marketing and Sales
  - BI and Analytics
  - CRM
- Each category has repeatable structure (benefit story, templates, use cases, integrations, FAQ).
- Local does not yet expose a dedicated "market intelligence" information architecture with this granularity.

### C. Connector-level prompt packaging
- Base44 connector pages provide "example prompts" that directly translate use case to build instruction.
- Local currently lacks a connector-page system that turns integrations into prompt-ready JTBD playbooks.

### D. Marketplace proof loop
- Base44 template pages show creator identity + clone counts.
- This creates public demand proof and social validation loops.
- Local does not visibly emphasize comparable public proof metrics across template/use-case assets.

### E. Public velocity signaling
- Base44 changelog is extensive and very visible in marketing navigation.
- Local changelog exists, but broader SEO-linked release storytelling (across category clusters) is less developed.

## 4) Reorganization proposal for Market Intelligence (recommended IA)

## Hub: /market-intelligence
- Mission: central intelligence layer for buyer-facing and product-facing signals.
- Content blocks:
  - Segment map
  - Use-case map
  - Connector map
  - Competitor signal feed
  - Release velocity feed
  - Template demand signals

## Pillar 1: Segments
- /market-intelligence/segments/marketing-sales
- /market-intelligence/segments/business-intelligence
- /market-intelligence/segments/crm
- Required schema per segment entry:
  - Persona
  - Job to be done
  - Trigger event
  - Core workflow
  - Required integrations
  - Security/compliance requirements
  - Budget sensitivity
  - KPI outcome

## Pillar 2: Use Cases
- /market-intelligence/use-cases/<slug>
- Start with 16 high-signal slugs from Base44 categories:
  - email-campaigns
  - landing-page-builder
  - sales-outreach
  - ad-copy-generator
  - social-media-scheduler
  - lead-nurturing
  - marketing-analytics
  - sales-dashboard
  - operations-reporting
  - customer-insights
  - lead-scoring
  - contact-management
  - sales-pipeline
  - client-onboarding
  - support-ticketing
  - project-management

## Pillar 3: Integrations Intelligence
- /market-intelligence/integrations/<connector>
- Prioritize:
  - hubspot
  - salesforce
  - linkedin
  - google-sheets
  - slack
  - zapier
- Required schema:
  - Connector capability summary
  - Example prompt patterns
  - Data flow direction (read/write/bidirectional)
  - Typical activation trigger
  - Common failure/risk notes

## Pillar 4: Competitor Signals
- /market-intelligence/competitors/base44
- Track weekly deltas:
  - New changelog entries
  - New template categories
  - New top templates by clone momentum
  - New/updated connector pages
  - New use-case pages

## Pillar 5: Demand Signals and Proof
- /market-intelligence/signals/templates
- /market-intelligence/signals/releases
- /market-intelligence/signals/content
- Goal: convert external signals into internal prioritization.

## 5) Suggested rollout (fast)

### Week 1
- Create market-intelligence hub + 3 segment pages + 6 integration pages.
- Add one normalized frontmatter schema for all intelligence pages.

### Week 2
- Create 16 use-case pages and cross-link to relevant connectors.
- Add comparison snippets against competitor framing (non-copy, original wording).

### Week 3
- Add competitor delta tracker page and lightweight update ritual.
- Add internal scorecards: opportunity score, execution effort, confidence.

## 6) Practical next artifact to build
- A structured JSON/MD registry file for all intelligence nodes:
  - id
  - type (segment/use-case/integration/competitor-signal)
  - primary_keyword
  - buyer_stage
  - related_connectors
  - related_pages
  - priority
  - last_updated

This registry will let you drive content, comparison pages, and product prioritization from one source of truth.
