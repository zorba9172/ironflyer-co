import type { IconName } from '../../icons';
import { neon } from '../../theme';

// ─────────────────────────────────────────────────────────────────────────
// Templates page data model.
//
// The six starter blueprints are unchanged in meaning from the original page
// (name, category, description, stack) — the live `use(name)` handler still
// calls startFromPrompt(`Start from the ${name} template`). This module only
// carries presentational metadata so each card reads as a proof surface:
//   • `icon`  — a semantic name resolved through the studio Icon barrel
//               (Iconography Law: never a vendor glyph import in a component);
//               it is the centered mark on the card's flat 2D thumbnail.
//   • `accent`— a studio `neon` token value (the sanctioned non-sx token bag),
//               used only to tint card chrome through the theme.
// No new app structure, routes, or data flow.
// ─────────────────────────────────────────────────────────────────────────

export type Template = {
  name: string;
  cat: string;
  desc: string;
  stack: string;
  /** semantic glyph name from the studio Icon barrel */
  icon: IconName;
  /** studio neon accent token used to tint the card chrome */
  accent: string;
  /** Glanceable proof stat shown on the card footer. */
  gates: number;
  /** Relative readiness of the starter (drives the meter + sort weight). */
  readiness: number;
  /** Short tag describing the typical ship horizon. */
  ships: string;
};

export const CATEGORIES = [
  'All',
  'SaaS',
  'Commerce',
  'AI',
  'Internal',
  'Marketing',
] as const;

export const TEMPLATES: readonly Template[] = [
  {
    name: 'SaaS dashboard',
    cat: 'SaaS',
    desc: 'Auth, billing, team roles, and an admin panel.',
    stack: 'React · Go · Postgres',
    icon: 'dashboard',
    accent: neon.violet,
    gates: 6,
    readiness: 92,
    ships: 'Ships in ~1 day',
  },
  {
    name: 'Marketplace',
    cat: 'Commerce',
    desc: 'Listings, Stripe payments, and seller payouts.',
    stack: 'React · Stripe · Postgres',
    icon: 'store',
    accent: neon.blue,
    gates: 7,
    readiness: 88,
    ships: 'Ships in ~2 days',
  },
  {
    name: 'AI chatbot',
    cat: 'AI',
    desc: 'Streaming chat, memory, and usage metering.',
    stack: 'React · streaming · ledger',
    icon: 'bot',
    accent: neon.pink,
    gates: 5,
    readiness: 95,
    ships: 'Ships in hours',
  },
  {
    name: 'Booking app',
    cat: 'Commerce',
    desc: 'Calendar, reminders, and Stripe checkout.',
    stack: 'React · Stripe · email',
    icon: 'schedule',
    accent: neon.purple,
    gates: 6,
    readiness: 86,
    ships: 'Ships in ~1 day',
  },
  {
    name: 'Internal tool',
    cat: 'Internal',
    desc: 'Tables, roles, and an audit log.',
    stack: 'React · RBAC · audit log',
    icon: 'wrench',
    accent: neon.success,
    gates: 5,
    readiness: 90,
    ships: 'Ships in hours',
  },
  {
    name: 'Landing + waitlist',
    cat: 'Marketing',
    desc: 'SEO pages, email capture, and analytics.',
    stack: 'React · SEO · analytics',
    icon: 'build',
    accent: neon.warning,
    gates: 4,
    readiness: 97,
    ships: 'Ships in minutes',
  },
];
