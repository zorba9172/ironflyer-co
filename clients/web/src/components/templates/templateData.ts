// templateData — the fallback catalogue rendered by the /templates
// gallery (and the home TemplatesGalleryPreview rail) when the
// orchestrator's `useBlueprintsQuery` returns no rows. The list below
// is the source of truth for the cockpit-grade demo experience:
// curated cards that match the categories the Templates page exposes
// in its filter chips.
//
// Source of truth: the eight templates here mirror the Base44
// "starter blueprint" gallery that ships with the Ironflyer brand
// system. When the orchestrator catalogue ships real blueprints
// (core/orchestrator/internal/blueprints/), the gallery prefers the
// live data; this list is the no-data fallback so the surface never
// reads empty during local development or unauthenticated browsing.

export type TemplateCategory =
  | "internal-tools"
  | "customer-apps"
  | "workflows"
  | "dashboards"
  | "api-services";

export interface TemplateData {
  slug: string;
  name: string;
  description: string;
  category: TemplateCategory;
  tags: string[];
  featured?: boolean;
}

export const TEMPLATE_CATEGORIES: {
  value: TemplateCategory | "all";
  label: string;
}[] = [
  { value: "all", label: "All" },
  { value: "internal-tools", label: "Internal tools" },
  { value: "customer-apps", label: "Customer apps" },
  { value: "workflows", label: "Workflows" },
  { value: "dashboards", label: "Dashboards" },
  { value: "api-services", label: "API services" },
];

export const FALLBACK_TEMPLATES: TemplateData[] = [
  {
    slug: "operations-dashboard",
    name: "Operations dashboard",
    description:
      "Live KPI tiles, drilldowns, role-aware filters and exportable reporting for ops teams.",
    category: "dashboards",
    tags: ["dashboards", "analytics", "reporting"],
    featured: true,
  },
  {
    slug: "client-portal",
    name: "Client portal",
    description:
      "Projects, files, approvals and customer-facing status pages with magic-link sign in.",
    category: "customer-apps",
    tags: ["portal", "auth", "files"],
  },
  {
    slug: "ai-chat-support",
    name: "AI chat support",
    description:
      "Retrieval-augmented helpdesk: knowledge base ingest, threaded inbox and human handoff.",
    category: "customer-apps",
    tags: ["ai", "support", "rag"],
  },
  {
    slug: "internal-crm",
    name: "Internal CRM",
    description:
      "Contacts, pipelines, notes and activity timelines wired to your existing identity provider.",
    category: "internal-tools",
    tags: ["crm", "pipelines", "sales"],
  },
  {
    slug: "stripe-billing-portal",
    name: "Stripe billing portal",
    description:
      "Plans, invoices, dunning UI and webhook handlers backed by a Stripe-shaped ledger.",
    category: "customer-apps",
    tags: ["billing", "stripe", "saas"],
  },
  {
    slug: "approval-workflow",
    name: "Approval workflow",
    description:
      "Multi-step approvals with audit trail, escalations and Slack / email notifications.",
    category: "workflows",
    tags: ["workflow", "approvals", "audit"],
  },
  {
    slug: "forms-and-approvals",
    name: "Forms & approvals",
    description:
      "Embedded forms, conditional logic and an approval queue with one-click verdicts.",
    category: "workflows",
    tags: ["forms", "intake", "queue"],
  },
  {
    slug: "analytics-dashboard",
    name: "Analytics dashboard",
    description:
      "Cohort retention, funnel views and SQL-backed metric tiles with scheduled refresh.",
    category: "dashboards",
    tags: ["analytics", "sql", "cohorts"],
  },
];

// Convert a live orchestrator blueprint row (Blueprints query) into the
// presentational TemplateData shape used by the cockpit cards. The
// orchestrator's `category` field is a free-form string, so we
// best-effort map it to one of the cockpit categories — unknown maps
// fall through to "customer-apps" so the card still renders inside
// the chip filter.
export function mapBlueprintCategoryToTemplateCategory(
  raw: string,
): TemplateCategory {
  const c = raw.toLowerCase();
  if (c.includes("dashboard") || c.includes("analytic")) return "dashboards";
  if (c.includes("api") || c.includes("service")) return "api-services";
  if (c.includes("workflow") || c.includes("approval")) return "workflows";
  if (c.includes("internal") || c.includes("ops") || c.includes("admin")) {
    return "internal-tools";
  }
  return "customer-apps";
}

// sessionStorage key — when the user opens a template, we drop the
// slug here so the Studio composer can prefill the prompt and pin the
// blueprint without relying on a query string round-trip.
export const TEMPLATE_PICK_SESSION_KEY = "ironflyer.studio.templatePick";

export interface TemplatePickPayload {
  slug: string;
  name: string;
  description: string;
}

export function persistTemplatePick(payload: TemplatePickPayload): void {
  if (typeof window === "undefined") return;
  try {
    window.sessionStorage.setItem(
      TEMPLATE_PICK_SESSION_KEY,
      JSON.stringify(payload),
    );
  } catch {
    // sessionStorage may be disabled (private mode); the query-string
    // form on the link is the canonical fallback.
  }
}
