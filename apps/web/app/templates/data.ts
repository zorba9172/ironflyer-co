// Templates manifest for the public marketplace. Each entry maps to a
// directory under /templates in the repo root and ships an `ideaPrompt`
// that the workspace pre-seeds when a visitor clicks "Use template".
//
// Authored by hand so the marketing surface stays curated — the on-disk
// /templates tree carries dozens of starters but only a representative
// subset belongs on the landing page. When you add an entry here, make
// sure the slug matches the directory name the orchestrator scaffolders
// resolve.

export type TemplateTag =
  | 'SaaS'
  | 'E-commerce'
  | 'Game'
  | 'Mobile'
  | 'Dashboard'
  | 'AI'
  | 'Social'
  | 'Learning'
  | 'Marketing'
  | 'Internal'
  | 'API'
  | 'Realtime';

export interface TemplateMeta {
  slug: string;        // matches the directory name under /templates
  name: string;
  tag: TemplateTag;
  description: string; // 1-line summary
  stack: string;       // e.g. "Next.js + Supabase + Stripe"
  gradient: string;    // CSS background gradient for the cover card
  ideaPrompt: string;  // pre-filled prompt that seeds the new project
  inside: string[];    // bullet list rendered on the SEO landing
}

export const TEMPLATES: TemplateMeta[] = [
  {
    slug: 'saas-pulseanalytics',
    name: 'Pulse Analytics',
    tag: 'SaaS',
    description: 'Multi-tenant SaaS with auth, teams, Stripe billing, and a usage dashboard.',
    stack: 'Next.js + Postgres + Stripe',
    gradient: 'linear-gradient(135deg, #1f3a8a 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build a production-ready multi-tenant SaaS analytics app. Email + password auth with team invites, Stripe subscription billing with three tiers, usage events ingested via REST, a dashboard with daily active users and event funnels, an admin console for plan overrides, and a one-click deploy gate.',
    inside: [
      'Email + password auth with email verification and team invites',
      'Stripe Checkout + customer portal wired through the billing gate',
      'Usage event ingest endpoint with rate limiting and per-tenant quotas',
      'Daily active users + funnel dashboard with date-range filters',
      'Admin console with plan overrides, refunds, and audit log',
      'Spec, Code, Security, and Budget gates enabled by default',
    ],
  },
  {
    slug: 'ecommerce-bloom',
    name: 'Bloom Storefront',
    tag: 'E-commerce',
    description: 'Headless storefront with cart, checkout, and inventory in one repo.',
    stack: 'Next.js + Stripe + Postgres',
    gradient: 'linear-gradient(135deg, #b3265c 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build a production e-commerce storefront. Product catalog with categories and search, cart persisted to the user account, Stripe Checkout for one-time payments, address + tax calculation, order history with status tracking, inventory deduction on order placement, and a minimal admin to add products and view orders.',
    inside: [
      'Product catalog with categories, search, and SEO-friendly slugs',
      'Persistent cart tied to the user account or guest session',
      'Stripe Checkout with address collection and tax rounding',
      'Inventory deduction on order placement with race-safe writes',
      'Customer order history page with status timeline',
      'Admin surface for product CRUD and order management',
    ],
  },
  {
    slug: 'game-phaser-arcade',
    name: 'Phaser Arcade',
    tag: 'Game',
    description: 'Browser game scaffold with scenes, score, and high-score leaderboard.',
    stack: 'Phaser 3 + TypeScript + Postgres',
    gradient: 'linear-gradient(135deg, #6d28d9 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build a browser arcade game on Phaser 3. Title scene, gameplay scene with player input, score tracking, game-over scene with name entry, server-backed leaderboard with top 100 scores, share button that copies a result URL, and mobile touch controls. Include the Phaser scaffolder and the JS/TS Code gate.',
    inside: [
      'Phaser 3 scene graph: title, gameplay, pause, game-over',
      'Input layer that handles keyboard, gamepad, and mobile touch',
      'Server-backed leaderboard with top 100 scores per game mode',
      'Shareable result URL that prefills the title scene',
      'Asset preloader with progress bar wired into the loop',
      'Phaser scaffolder + JS/TS code gate enabled out of the box',
    ],
  },
  {
    slug: 'mobile-expo-fitness',
    name: 'Expo Fitness',
    tag: 'Mobile',
    description: 'Cross-platform mobile app with auth, workouts, and offline sync.',
    stack: 'Expo + React Native + SQLite',
    gradient: 'linear-gradient(135deg, #047857 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build a cross-platform mobile fitness app on Expo. Phone-OTP login, daily workout plan, exercise timer, progress charts, offline-first local SQLite with cloud sync when online, push notifications for streaks, and a profile screen. Use the Expo scaffolder and enable the Mobile build gate.',
    inside: [
      'Expo managed workflow with iOS + Android targets',
      'Phone-OTP login flow with secure refresh tokens',
      'Daily workout plan engine with rest-day logic',
      'Offline-first SQLite with conflict-aware cloud sync',
      'Streak push notifications via Expo Notifications',
      'Mobile build gate produces signed artifacts on every release',
    ],
  },
  {
    slug: 'dashboard-orbitcrm',
    name: 'Orbit CRM',
    tag: 'Dashboard',
    description: 'Internal CRM with pipeline, deals, and activity feed across teams.',
    stack: 'Next.js + Postgres + Server Actions',
    gradient: 'linear-gradient(135deg, #0f766e 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build an internal CRM dashboard. Sales pipeline with drag-and-drop deal stages, contact list with company rollups, weekly activity feed per rep, role-based access for SDR / AE / Admin, CSV import for leads, and a saved-views system for filters. Use Postgres, Server Actions, and the Internal Tool scaffold.',
    inside: [
      'Pipeline board with drag-and-drop stages and won/lost flags',
      'Contact + company entities with bi-directional rollups',
      'Activity feed per rep with notes, calls, and meeting logs',
      'Role-based access: SDR, AE, Admin — checked in middleware',
      'CSV importer with column mapping and error report',
      'Saved-views system so each rep keeps their favorite filters',
    ],
  },
  {
    slug: 'ai-synthwave-chatbot',
    name: 'Synthwave Chatbot',
    tag: 'AI',
    description: 'Retrieval chatbot over your docs with session memory and analytics.',
    stack: 'Next.js + pgvector + Anthropic SDK',
    gradient: 'linear-gradient(135deg, #be185d 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build an AI customer-support chatbot. Doc upload with chunking and pgvector embeddings, retrieval over the indexed corpus, streaming chat UI with session memory, fallback to a human-handoff queue when confidence is low, an admin analytics dashboard with deflection rate, and per-session cost tracking against the budget ledger.',
    inside: [
      'Doc uploader with chunking, dedupe, and pgvector embeddings',
      'Retrieval-augmented chat with citations on every answer',
      'Streaming UI that respects the BillingGuard token caps',
      'Human-handoff queue with priority and notes',
      'Admin analytics: deflection rate, top intents, unanswered',
      'Per-session cost tracking written to the budget ledger',
    ],
  },
  {
    slug: 'social-yardbird',
    name: 'Yardbird Feed',
    tag: 'Social',
    description: 'Social feed app with posts, follows, likes, and media uploads.',
    stack: 'Next.js + Postgres + S3-compatible storage',
    gradient: 'linear-gradient(135deg, #c2410c 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build a social feed app. Email signup with profile setup, follow graph, posts with image upload to S3-compatible storage, likes and reply threads, a notifications page, an infinite-scroll feed with cursor pagination, and a basic moderation queue for reported posts.',
    inside: [
      'Profile setup flow with handle, bio, and avatar upload',
      'Follow graph with reciprocal-follow detection',
      'Image upload via signed URLs to S3-compatible storage',
      'Infinite-scroll feed with cursor pagination',
      'Notifications inbox with read/unread state',
      'Moderation queue for reported posts with audit trail',
    ],
  },
  {
    slug: 'learning-beacon-academy',
    name: 'Beacon Academy',
    tag: 'Learning',
    description: 'Learning platform with courses, lessons, quizzes, and progress tracking.',
    stack: 'Next.js + Postgres + Stripe',
    gradient: 'linear-gradient(135deg, #1d4ed8 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build a learning platform. Course catalog with categories, lesson player supporting video and markdown, quizzes with auto-grading, learner progress tracking, certificate of completion, instructor dashboard with revenue share, and Stripe billing for per-course purchases or all-access subscriptions.',
    inside: [
      'Course + lesson + quiz schema with publish lifecycle',
      'Lesson player supporting video, markdown, and embedded code',
      'Auto-graded quizzes with question banks and randomization',
      'Progress tracking with resume-where-you-left-off',
      'Instructor dashboard with revenue share and payouts',
      'Stripe billing: per-course purchases or all-access subscription',
    ],
  },
  {
    slug: 'marketing-launchpad',
    name: 'Launchpad Site',
    tag: 'Marketing',
    description: 'Marketing site with hero, waitlist, pricing, FAQ, and analytics events.',
    stack: 'Next.js + Server Actions + Resend',
    gradient: 'linear-gradient(135deg, #92400e 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build a marketing launch site. Hero with waitlist form that sends a confirmation email via Resend, pricing section with three tiers, FAQ with anchor links, blog with MDX, sitemap.xml + robots.txt, OpenGraph images for every page, and analytics events wired into the dataLayer for downstream tools.',
    inside: [
      'Hero with waitlist form and double opt-in via Resend',
      'Three-tier pricing block with annual/monthly toggle',
      'FAQ with deep-link anchors and JSON-LD for SEO',
      'MDX blog with tag pages and RSS feed',
      'Auto-generated sitemap.xml, robots.txt, and OG images',
      'dataLayer events that downstream tools can consume',
    ],
  },
  {
    slug: 'internal-ledgerpro',
    name: 'LedgerPro Ops',
    tag: 'Internal',
    description: 'Internal ops tool with approvals, audit trail, and dense tables.',
    stack: 'Next.js + Postgres + Server Actions',
    gradient: 'linear-gradient(135deg, #334155 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build an internal operations tool. Role-based approvals with required reviewers, dense tabular UI with column resize and CSV export, full audit history per record, search across entities, scheduled exports via cron, and SSO-ready auth. Use the Internal Tool scaffold.',
    inside: [
      'Role-based approvals with two-of-three reviewer rules',
      'Dense table UI with column resize, sort, and CSV export',
      'Append-only audit history per record with diff viewer',
      'Cross-entity search backed by a Postgres trigram index',
      'Scheduled exports via cron with delivery to email or S3',
      'SSO-ready auth (OIDC) wired through the auth middleware',
    ],
  },
  {
    slug: 'api-routecloud',
    name: 'RouteCloud API',
    tag: 'API',
    description: 'Go HTTP service with versioned routes, OpenAPI, and rate limiting.',
    stack: 'Go + chi + Postgres',
    gradient: 'linear-gradient(135deg, #0e7490 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build a production Go HTTP API. chi router with versioned /v1 routes, JWT auth, per-key rate limiting backed by Redis, OpenAPI 3 spec generated from handlers, structured zerolog logs, Prometheus metrics, and an integration-test suite. Use the Go scaffolder and enable the Go code gate.',
    inside: [
      'chi router with versioned /v1 prefix and middleware chain',
      'JWT auth + per-API-key rate limit backed by Redis',
      'OpenAPI 3 spec generated from handler annotations',
      'Structured zerolog with request IDs and PII redaction',
      'Prometheus metrics endpoint + Grafana-ready dashboards',
      'Go code gate + integration-test gate enabled by default',
    ],
  },
  {
    slug: 'realtime-inboxzero',
    name: 'InboxZero Chat',
    tag: 'Realtime',
    description: 'Realtime team chat with channels, presence, and typing indicators.',
    stack: 'Next.js + WebSockets + Postgres',
    gradient: 'linear-gradient(135deg, #0f172a 0%, #0d0e0f 100%)',
    ideaPrompt:
      'Build a realtime team chat. Channels and direct messages, presence and typing indicators over WebSockets, message search with full-text indexing, file attachments, push notifications on mention, unread counts per channel, and an admin console for workspace settings. Use the Realtime scaffold.',
    inside: [
      'WebSocket gateway with presence and typing indicators',
      'Channels + DMs with reactions and threaded replies',
      'Full-text search on message body with Postgres tsvector',
      'File attachments via signed URLs and inline previews',
      'Mention-driven push notifications and unread counts',
      'Admin console for workspace settings and member roles',
    ],
  },
];

export function getTemplate(slug: string): TemplateMeta | undefined {
  return TEMPLATES.find((t) => t.slug === slug);
}

export const TEMPLATE_TAGS: TemplateTag[] = [
  'SaaS',
  'E-commerce',
  'Game',
  'Mobile',
  'Dashboard',
  'AI',
  'Social',
  'Learning',
  'Marketing',
  'Internal',
  'API',
  'Realtime',
];
