import type { IconType } from 'react-icons';
import {
  LuLayoutDashboard,
  LuStore,
  LuBot,
  LuCalendarClock,
  LuWrench,
  LuRocket,
} from 'react-icons/lu';
import { neon } from '../../theme';

// ─────────────────────────────────────────────────────────────────────────
// Templates page data model.
//
// The six starter blueprints are unchanged from the original page (name,
// category, description, stack) — the live `use(name)` handler still calls
// startFromPrompt(`Start from the ${name} template`). This module only adds
// presentational metadata (a neon accent + icon + glanceable stats) so each
// card can read as a proof surface instead of a flat list row. No new app
// structure, routes, or data flow.
// ─────────────────────────────────────────────────────────────────────────

export type Template = {
  name: string;
  cat: string;
  desc: string;
  stack: string;
  Icon: IconType;
  accent: string;
  /** Glanceable proof stats shown on the card footer. */
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
    Icon: LuLayoutDashboard,
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
    Icon: LuStore,
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
    Icon: LuBot,
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
    Icon: LuCalendarClock,
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
    Icon: LuWrench,
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
    Icon: LuRocket,
    accent: neon.warning,
    gates: 4,
    readiness: 97,
    ships: 'Ships in minutes',
  },
];
