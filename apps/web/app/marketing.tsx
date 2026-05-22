import Link from 'next/link';
import {
  ArrowForward, BarChart, CheckCircle, CloudQueue, Code, EditNote,
  Apps as AppsIcon, Groups, Hub, Inventory2, Lock, RocketLaunch, Security, ShoppingCart, Tune,
  VerifiedUser, Work,
} from '@mui/icons-material';
import {
  Box, Button, Chip, Container, Divider, Stack, Typography,
} from '@mui/material';
import { tokens } from '../../../packages/design-tokens';
import { EnterpriseLeadForm } from './enterprise-lead-form';
import { HeroQuickStarts } from './hero-quick-starts';
import { PromptBox } from './prompt-box';
import { UpgradeButton } from './upgrade-button';

const imageBase = '/marketplace/output-ref';

const navItems = [
  { label: 'Product', href: '/product' },
  { label: 'Templates', href: '/templates' },
  { label: 'Pricing', href: '/pricing' },
  { label: 'Solutions', href: '/solutions' },
  { label: 'Security', href: '/security' },
  { label: 'Enterprise', href: '/enterprise' },
];

const steps = [
  { label: 'Spec it', text: 'Turn a raw idea into user stories, data shape, and product boundaries.' },
  { label: 'Build it', text: 'Spin up the interface, backend contracts, runtime workspace, and first code pass.' },
  { label: 'Gate it', text: 'Run UX, architecture, tests, security, budget, and deploy readiness checks.' },
  { label: 'Ship it', text: 'Keep iterating until the release is coherent enough to leave the workshop.' },
];

const gates = ['Spec', 'UX', 'Architecture', 'Code', 'Tests', 'Security', 'Deploy'];

// Four-card feature tour shown right after the hero. Each card carries one
// promise the user buys into: speed, backend depth, deploy-readiness, and
// model freedom. The structure mirrors Base44's landing layout but every
// promise is sharpened with Ironflyer's finisher angle so the comparison
// shopper sees what we do that they don't.
// FAQs mirror Base44's landing-page Q&A but lead each answer with what
// makes Ironflyer's behaviour different — finisher gates, real Linux
// sandbox, multi-provider routing, transparent margin model. Order is
// chosen so the comparison shopper hits "what makes you different" first
// and "how do credits work" before security / ownership.
// Quick-start chips shown right under the hero PromptBox. Clicking one
// seeds the dashboard's pendingIdea (read by /app/page.tsx) and routes
// the user to the workspace — Base44's template-chip pattern with our
// finisher-shaped seeds. Keep the list short; longer lists belong on
// /templates.
// Use-case grids mirror Base44's "By Industry" + "By Role" landing
// surface. Each row lists ~6 slots; the visitor scans the row matching
// their identity and clicks through to /solutions filtered by tag. We
// keep the labels short — the page is for self-identification, not
// reading.
// Comparison table — names the three competitors visitors are likely
// shopping against (Base44, Lovable, Bolt.new) and rows the differences
// the finisher gates create. Honest where the competitor edges us (e.g.
// Lovable's Visual Edits sidebar) so the table reads credibly.
const comparisonRows: { label: string; values: [string, string, string, string] }[] = [
  {
    label: 'Spec / UX / Code gates enforced',
    values: ['✓ All 8', '—', '—', '—'],
  },
  {
    label: 'Self-managing budget (sub − cost = margin)',
    values: ['✓ Live $ + cap', 'Credit packs', 'Credit packs', 'Token bucket'],
  },
  {
    label: 'Real per-user Linux sandbox + PTY',
    values: ['✓ Docker driver', 'Hosted runtime', 'Hosted runtime', 'WebContainer'],
  },
  {
    label: 'Multi-provider routing (Anthropic + OpenAI + on-device)',
    values: ['✓ By capability', '—', 'Internal models', 'Frontier coder'],
  },
  {
    label: 'Effort dial (Lite / Economy / Power)',
    values: ['✓', '—', '—', '—'],
  },
  {
    label: 'Bring-your-own cloud via Helm',
    values: ['✓ Chart shipped', '—', '—', '—'],
  },
  {
    label: 'GitHub bi-directional push',
    values: ['✓', '✓', '✓', '✓'],
  },
  {
    label: 'Visual click-to-edit sidebar',
    values: ['Coming Q3', '—', '✓', '—'],
  },
];

const useCasesByIndustry = [
  { tag: 'productivity', label: 'Productivity' },
  { tag: 'education',    label: 'Education' },
  { tag: 'entertainment', label: 'Entertainment' },
  { tag: 'health',       label: 'Health & wellness' },
  { tag: 'commerce',     label: 'E-commerce' },
  { tag: 'finance',      label: 'Finance' },
];

const useCasesByRole = [
  { tag: 'product',     label: 'Product Management' },
  { tag: 'operations',  label: 'Operations' },
  { tag: 'marketing',   label: 'Marketing & Sales' },
  { tag: 'hr',          label: 'HR & Recruitment' },
  { tag: 'engineering', label: 'Dev Productivity' },
  { tag: 'analytics',   label: 'Business Intelligence' },
];

const heroQuickStarts = [
  { label: 'Internal tool', prompt: 'Build an internal operations tool with approvals, role-based access, audit history, reports, and a dense dashboard UI.' },
  { label: 'SaaS dashboard', prompt: 'Build a production-ready SaaS app with auth, teams, billing, analytics, admin settings, onboarding, and deploy.' },
  { label: 'Client portal', prompt: 'Build a client portal with authentication, document uploads, project status, messaging, and admin controls.' },
  { label: 'Launch site', prompt: 'Build a product launch website with hero, waitlist, pricing, FAQ, social proof, and analytics events.' },
  { label: 'Marketplace', prompt: 'Build a two-sided marketplace with listings, search filters, messaging, escrow payouts via Stripe, and trust scoring.' },
];

const faqs = [
  {
    q: 'How is Ironflyer different from Base44, Lovable, or Bolt?',
    a: 'Generators ship code straight to preview; Ironflyer ships through gates — Spec, UX, Architecture, Code, Lint, Tests, Security, Deploy — and the loop refuses to publish until every one passes. You get a finished product, not a demo.',
  },
  {
    q: 'Do I need to know how to code?',
    a: 'No. Describe what you want and the agents do the work. Coders and product folks get extra leverage: every project is a real repo with a real Linux sandbox, so you can drop into the IDE, run a terminal, and edit anything by hand whenever you want.',
  },
  {
    q: 'What do I get on the Free plan?',
    a: 'All core features — auth, database, build credits, the cloud IDE, public templates. The Ironflyer badge stays on, projects are public, and credits are capped so you can never overspend. Upgrade for private projects, custom domains, and bigger credit pools.',
  },
  {
    q: 'How does the credit / budget system work?',
    a: 'Every plan has a monthly subscription and a measured cost cap — what Ironflyer is willing to absorb in provider spend. The Budget card shows live $ burn against the cap and the top three models you’re paying for. No credit traps, no surprise bills: when the cap is reached the enforcer downgrades or pauses cleanly.',
  },
  {
    q: 'Which AI models does Ironflyer use?',
    a: 'A multi-provider router picks the cheapest model that satisfies the capability tags on each agent call — Anthropic, OpenAI, or on-device ONNX. The Lite / Economy / Power dial in chat lets you override the bias yourself when you care about cost or depth.',
  },
  {
    q: 'Can I bring this to my own cloud?',
    a: 'Yes. The whole stack ships as a Helm chart you install in any Kubernetes cluster — orchestrator, runtime sandboxes, web, Postgres, optional ingress + TLS. The DEPLOY.md runbook walks you through it.',
  },
  {
    q: 'How do you handle security?',
    a: 'A Security gate scans for credentials, dependency drift, and OWASP-class issues on every iteration; the cloud IDE runs in a per-user Docker sandbox, never shared; secrets land in a Kubernetes Secret you control; and every gate write goes through a patch lifecycle — the AI never mutates files directly.',
  },
  {
    q: 'Who owns the code?',
    a: 'You. Connect a GitHub repo and the loop pushes there; export the project from the workspace and you walk away with a full repo plus the Dockerfile and Helm chart. No platform lock-in on the artefact.',
  },
];

const capabilityTourCards = [
  {
    eyebrow: '01 · Idea to app',
    title: 'At the speed of finish, not just thought.',
    text:
      'Describe what you want. Ironflyer plans the spec, generates the UX, writes the code, runs your tests, and packages a deploy — gated end-to-end.',
    chip: 'Finisher loop',
  },
  {
    eyebrow: '02 · Backend done',
    title: 'Auth, database, files, exec — wired before line one.',
    text:
      'Users, sessions, roles, Postgres schemas, a real per-user Linux sandbox with PTY, and a budget ledger ship live on day one. No Supabase tab. No Vercel handoff.',
    chip: 'Already wired',
  },
  {
    eyebrow: '03 · Production from start',
    title: 'Custom domain, deploy gate, observability.',
    text:
      'Helm chart, Prometheus metrics, healthchecks, secrets management, and a Deploy gate that refuses to ship until tests + security pass. Your first commit is already prod-shaped.',
    chip: 'Day one ready',
  },
  {
    eyebrow: '04 · Any model, every gate',
    title: 'Multi-provider routing with a hand on the dial.',
    text:
      'Anthropic, OpenAI, on-device — the provider router picks the cheapest model that satisfies the gate. The Lite / Economy / Power dial in chat puts the cost-quality call back in your hand.',
    chip: 'Open routing',
  },
];

const productTiles = [
  {
    title: 'AI Product Finisher',
    text: 'A structured build loop for teams who care less about a pretty prototype and more about arriving at a real product.',
    image: `${imageBase}/arcade.jpg`,
    accent: tokens.color.accent.lime,
  },
  {
    title: 'Multi-agent workspace',
    text: 'Planner, UX, architect, coder, tester, security and deploy roles work against the same project state.',
    image: `${imageBase}/fx.png`,
    accent: tokens.color.accent.sky,
  },
  {
    title: 'Patch-safe runtime',
    text: 'Files, terminal, preview, budget and execution events sit together so every change has context.',
    image: `${imageBase}/pack-generator.png`,
    accent: tokens.color.accent.coral,
  },
];

const blueprints = [
  { title: 'SaaS dashboard', label: 'Subscription product', image: `${imageBase}/hooked.png` },
  { title: 'Internal ops tool', label: 'Workflow + approvals', image: `${imageBase}/fx.png` },
  { title: 'Client portal', label: 'Auth + documents', image: `${imageBase}/pack-generator.png` },
  { title: 'Launch site', label: 'Marketing + forms', image: `${imageBase}/gear.png` },
];

const templateCategories = [
  { label: 'All Templates', icon: <AppsIcon /> },
  { label: 'Websites', icon: <CloudQueue /> },
  { label: 'Apps', icon: <Inventory2 /> },
  { label: 'SaaS', icon: <BarChart /> },
  { label: 'Internal Tools', icon: <Work /> },
  { label: 'Developer Tools', icon: <Code /> },
  { label: 'Editorial', icon: <EditNote /> },
  { label: 'Ecommerce', icon: <ShoppingCart /> },
];

const templateLibrary = [
  { title: 'SignalDesk', desc: 'Executive SaaS analytics dashboard with billing, usage, and admin roles', tag: 'Apps', image: `${imageBase}/hooked.png` },
  { title: 'OpsForge', desc: 'Internal approval workspace with teams, status boards, and audit history', tag: 'Internal Tools', image: `${imageBase}/fx.png` },
  { title: 'PortalKit', desc: 'Client portal with document exchange, comments, auth, and project status', tag: 'Websites', image: `${imageBase}/pack-generator.png` },
  { title: 'LaunchWave', desc: 'Product launch site with waitlist, pricing, FAQs, and conversion sections', tag: 'Websites', image: `${imageBase}/gear.png` },
  { title: 'Revenue Room', desc: 'Subscription analytics, seat management, invoices, and health scoring', tag: 'SaaS', image: `${imageBase}/arcade.jpg` },
  { title: 'EditorFlow', desc: 'Content operation dashboard for briefs, approvals, publishing, and metrics', tag: 'Editorial', image: `${imageBase}/hero.jpg` },
];

const plans = [
  {
    name: 'Free',
    price: '$0',
    period: '/month',
    text: 'Explore the loop, create a workspace, and validate whether the idea has a real product shape.',
    cta: 'Start free',
    badge: 'Start here',
    features: ['Starter build credits', 'Public templates', 'Basic gates', 'Ironflyer badge'],
  },
  {
    name: 'Pro',
    price: '$20',
    period: '/month',
    text: 'For founders and builders shipping real MVPs with budget controls and production checks.',
    cta: 'Go Pro',
    badge: 'Most popular',
    features: ['Monthly build credits', 'Private projects', 'Custom domains', 'Remove branding'],
  },
  {
    name: 'Team',
    price: '$40',
    period: '/month',
    text: 'For teams and agencies that need shared workspaces, reusable templates, and approval gates.',
    cta: 'Create team',
    badge: 'Scale together',
    features: ['Shared credit pool', 'Seats included', 'Roles and approvals', 'Template library'],
  },
  {
    name: 'Enterprise',
    price: 'Custom',
    period: '',
    text: 'For organizations that need SSO, audit logs, private deployment paths, and procurement support.',
    cta: 'Contact sales',
    badge: 'Procurement ready',
    features: ['SSO and audit logs', 'Private connectors', 'Dedicated onboarding', 'Custom limits and SLA'],
  },
];

const revenuePillars = [
  { title: 'Transparent usage', text: 'Buyers understand credits, caps, and what happens before spend grows.' },
  { title: 'Team expansion', text: 'Solo builders can become teams with seats, shared workspaces, and reusable templates.' },
  { title: 'Enterprise trust', text: 'SSO, audit trails, private connectors, and support create a path to larger contracts.' },
];

const pricingAssumptions = [
  { label: 'Free to paid', text: 'A useful free tier creates habit, then Pro removes limits and branding.' },
  { label: 'Credits with caps', text: 'Credits work when usage is visible and the product prevents surprise bills.' },
  { label: 'Annual commitment', text: 'Annual plans should be positioned as predictable capacity, not just a discount.' },
];

const solutionCards = [
  { title: 'Founders', text: 'Compress the path from idea to MVP without losing the product decisions that matter.' },
  { title: 'Product teams', text: 'Prototype, validate, and hand off software that already has structure behind it.' },
  { title: 'Agencies', text: 'Move client work through repeatable checkpoints before it becomes technical debt.' },
  { title: 'Internal tools', text: 'Build dense, useful workflows for operations, sales, finance, support, and people teams.' },
];

const footerGroups = [
  { title: 'Product', links: ['Platform', 'Templates', 'Pricing', 'Changelog', 'Status'] },
  { title: 'Solutions', links: ['Founders', 'Product Teams', 'Agencies', 'Internal Tools'] },
  { title: 'Company', links: ['Security', 'Enterprise', 'Contact', 'Support'] },
  { title: 'Legal', links: ['Privacy', 'Terms', 'DPA'] },
];

const panelSx = {
  borderRadius: { xs: 3, md: 5 },
  backgroundColor: '#e8dfce',
  overflow: 'hidden',
  color: '#080808',
};

const outputPageSx = {
  bgcolor: tokens.color.bg.alabaster,
  color: tokens.color.text.inverse,
};

export function MarketingShell({ children }: { children: React.ReactNode }) {
  return (
    <Box sx={{
      minHeight: '100vh',
      ...outputPageSx,
    }}>
      <SiteNav />
      {children}
      <SiteFooter />
    </Box>
  );
}

export function MarketingHome() {
  return (
    <MarketingShell>
      <HeroSection />
      <LogoBand />
      <CapabilityTour />
      <MeetSection />
      <RevenueEngineSection />
      <ProductShowcase />
      <BlueprintSection />
      <UseCaseGrid />
      <ComparisonTable />
      <NumbersSection />
      <FAQSection />
      <FinalCta />
    </MarketingShell>
  );
}

export function ProductPage() {
  return (
    <MarketingShell>
      <PageHero
        eyebrow="Platform"
        title="The product finisher loop"
        text="Ironflyer keeps planning, UX, code, tests, security, budget and deployment in one orchestrated workspace."
        image={`${imageBase}/arcade.jpg`}
      />
      <RevenueEngineSection />
      <ProductShowcase />
      <GateBand />
      <FinalCta />
    </MarketingShell>
  );
}

export function SolutionsPage() {
  return (
    <MarketingShell>
      <PageHero
        eyebrow="Solutions"
        title="For teams that need output, not theater"
        text="Use Ironflyer for MVPs, internal tools, SaaS dashboards, client portals, and workflow-heavy products."
        image={`${imageBase}/fx.png`}
      />
      <CardGrid items={solutionCards} icon={<Groups />} />
      <BlueprintSection />
      <FinalCta />
    </MarketingShell>
  );
}

export function TemplatesPage() {
  return (
    <MarketingShell>
      <TemplatesLibrarySection />
      <FinalCta />
    </MarketingShell>
  );
}

export function PricingPage() {
  return (
    <MarketingShell>
      <PageHero
        eyebrow="Pricing"
        title="Start small, scale with governance"
        text="Simple tiers for solo builders, teams, and organizations that need controls before software reaches production."
        image={`${imageBase}/gear.png`}
      />
      <RevenueEngineSection />
      <PricingSection />
      <FinalCta />
    </MarketingShell>
  );
}

export function SecurityPage() {
  const items = [
    { title: 'Role-based workspaces', text: 'Control who can create, run, approve, and deploy project changes.' },
    { title: 'Secrets and environments', text: 'Keep credentials out of prompts and isolate runtime configuration.' },
    { title: 'Security gates', text: 'Run checks before deployment and keep the issues attached to the project.' },
    { title: 'Audit trail', text: 'Track execution events, agent turns, budget entries, and release decisions.' },
  ];

  return (
    <MarketingShell>
      <PageHero
        eyebrow="Security"
        title="Secure by design, gated before deploy"
        text="Ironflyer treats security as a release checkpoint, not a paragraph near the bottom of a website."
        image={`${imageBase}/pack-generator.png`}
      />
      <CardGrid items={items} icon={<Security />} />
      <GateBand />
      <FinalCta />
    </MarketingShell>
  );
}

export function EnterprisePage() {
  const items = [
    { title: 'SSO and SCIM', text: 'Bring Ironflyer into an existing identity and employee lifecycle.' },
    { title: 'Custom connectors', text: 'Connect internal tools, repositories, design systems, and deployment targets.' },
    { title: 'Private deployments', text: 'Fit the runtime to your cloud, data boundaries, and compliance posture.' },
    { title: 'Onboarding services', text: 'Give teams a repeatable way to ship production software with AI assistance.' },
  ];

  return (
    <MarketingShell>
      <PageHero
        eyebrow="Enterprise"
        title="Finish production software with governance"
        text="For organizations that want AI speed, human approval, predictable controls, and a clean audit trail."
        image={`${imageBase}/fx.png`}
      />
      <CardGrid items={items} icon={<VerifiedUser />} />
      <EnterpriseLeadSection />
      <FinalCta primary="Book a product demo" secondary="Talk to engineering" />
    </MarketingShell>
  );
}

function SiteNav() {
  const navLinkSx = {
    color: '#111',
    fontWeight: 800,
    transition: `color ${tokens.motion.base} ${tokens.motion.curve}`,
    '&:hover': { color: '#6f7600' },
  };

  return (
    <Box component="header" sx={{ bgcolor: tokens.color.bg.alabaster }}>
      <Box sx={{ bgcolor: '#0d0e0f', color: tokens.color.bg.alabaster }}>
        <Container maxWidth="xl" sx={{
          minHeight: { xs: 54, md: 54 },
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: { xs: 1, md: 1.5 },
        }}>
          <Box component="img" src={`${imageBase}/pack-generator.png`} alt="" sx={{ width: { xs: 34, md: 42 }, height: { xs: 28, md: 34 }, objectFit: 'cover', borderRadius: 0.75 }} />
          <Typography variant="body2" sx={{ fontWeight: 900, fontSize: { xs: 13, md: 14 }, lineHeight: 1.15 }}>NEW: Ironflyer Product Finisher</Typography>
          <Link href="/app" style={{ color: tokens.color.accent.lime, textDecoration: 'none' }}>
            <Typography variant="body2" sx={{ fontWeight: 900, fontSize: { xs: 13, md: 14 }, lineHeight: 1.15 }}>Try it free</Typography>
          </Link>
          <ArrowForward sx={{ fontSize: 16, color: tokens.color.accent.lime }} />
        </Container>
      </Box>

      <Container maxWidth="xl" sx={{
        minHeight: { xs: 66, md: 70 },
        display: 'grid',
        gridTemplateColumns: { xs: '1fr auto', md: '1fr auto 1fr' },
        alignItems: 'center',
        gap: { xs: 1.5, md: 3 },
      }}>
        <Link href="/" style={{ color: 'inherit', textDecoration: 'none' }}>
          <Typography sx={{
            fontFamily: tokens.font.display,
            fontSize: { xs: 24, md: 28 },
            lineHeight: 1,
            textTransform: 'uppercase',
          }}>
            IRONFLYER
          </Typography>
        </Link>

        <Stack direction="row" spacing={{ md: 4, lg: 5 }} justifyContent="center" sx={{ display: { xs: 'none', md: 'flex' } }}>
          {navItems.slice(0, 5).map((item) => (
            <Link key={item.href} href={item.href} style={{ color: 'inherit', textDecoration: 'none' }}>
              <Typography variant="body2" sx={navLinkSx}>{item.label}</Typography>
            </Link>
          ))}
        </Stack>

        <Stack direction="row" spacing={1} alignItems="center" justifyContent="flex-end">
          <Button component={Link} href="/login" variant="text" sx={{ color: '#111', bgcolor: '#e9dfcd', px: 2, display: { xs: 'none', sm: 'inline-flex' }, '&:hover': { bgcolor: '#ddd2bd' } }}>
            Log in
          </Button>
          <Button component={Link} href="/app" variant="contained" endIcon={<ArrowForward />} sx={{ minWidth: { xs: 88, sm: 166 }, bgcolor: tokens.color.accent.lime, color: '#050505' }}>
            <Box component="span" sx={{ display: { xs: 'none', sm: 'inline' } }}>Get started</Box>
            <Box component="span" sx={{ display: { xs: 'inline', sm: 'none' } }}>Start</Box>
          </Button>
        </Stack>
      </Container>
    </Box>
  );
}

function HeroSection() {
  return (
    <Box component="main" sx={{
      bgcolor: tokens.color.bg.alabaster,
      pt: { xs: 0, md: 0 },
      pb: { xs: 7, md: 8 },
    }}>
      <Container maxWidth="xl">
        <Box sx={{
          minHeight: { xs: 620, md: 700 },
          borderRadius: { xs: 3, md: 5 },
          overflow: 'hidden',
          position: 'relative',
          display: 'grid',
          placeItems: 'center',
          px: { xs: 2, md: 6 },
          py: { xs: 5, md: 8 },
          backgroundImage: `linear-gradient(180deg, rgba(13,14,15,0.1), rgba(13,14,15,0.35)), url(${imageBase}/hero.jpg)`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
        }}>
          <Stack spacing={{ xs: 2.4, md: 3 }} alignItems="center" sx={{ width: '100%', textAlign: 'center', zIndex: 1 }}>
            <Chip label="New Better Shipping - Apps built to be finished" sx={{
              width: 'fit-content',
              bgcolor: 'rgba(244,240,232,0.92)',
              color: '#111',
              borderRadius: 999,
              fontWeight: 900,
              px: 0.75,
              '& .MuiChip-label': { px: 1.2 },
            }} />
            <Typography component="h1" variant="h1" sx={{
              fontSize: { xs: '3.2rem', sm: '5.4rem', md: '7.1rem' },
              lineHeight: 0.88,
              letterSpacing: 0,
              maxWidth: 900,
              color: tokens.color.bg.alabaster,
              textWrap: 'balance',
            }}>
              Build something <Box component="span" sx={{ color: tokens.color.accent.lime }}>finished</Box>
            </Typography>
            <Typography variant="h5" sx={{
              maxWidth: 760,
              color: tokens.color.bg.alabaster,
              fontSize: { xs: '1.05rem', md: '1.28rem' },
              fontWeight: 800,
              letterSpacing: 0,
              textShadow: '0 1px 14px rgba(0,0,0,0.28)',
            }}>
              Create apps and websites by chatting with AI
            </Typography>
            <Box sx={{ width: '100%', maxWidth: 820, pt: { xs: 1, md: 1.5 } }}>
              <PromptBox
                size="hero"
                cta="Build"
                placeholder="Describe the app or website you want to create..."
              />
              <HeroQuickStarts items={heroQuickStarts} />
            </Box>
          </Stack>
          <Box sx={{
            position: 'absolute',
            inset: 0,
            background: 'radial-gradient(circle at 50% 46%, rgba(0,0,0,0.05), rgba(0,0,0,0.28) 72%)',
          }} />
        </Box>
      </Container>
    </Box>
  );
}

function LogoBand() {
  const labels = ['Credit caps', 'Private projects', 'Team seats', 'SSO', 'Audit logs'];
  return (
    <Box sx={{ bgcolor: tokens.color.bg.alabaster, py: { xs: 5, md: 7 } }}>
      <Container maxWidth="lg">
        <Stack spacing={3} alignItems="center">
          <Typography variant="body2" sx={{ color: '#111', fontWeight: 700 }}>
            Built around the buying signals serious teams expect
          </Typography>
          <Stack direction="row" spacing={{ xs: 3, md: 8 }} useFlexGap flexWrap="wrap" justifyContent="center" alignItems="center">
            {labels.map((label) => (
              <Typography key={label} variant="h5" sx={{ fontWeight: 900, color: '#171717', opacity: 0.62 }}>
                {label}
              </Typography>
            ))}
          </Stack>
        </Stack>
      </Container>
    </Box>
  );
}

// CapabilityTour is the four-card promise grid right after the hero. Each
// card carries one buy-in line (speed / backend / production / model
// freedom) and ends in a chip that recalls the Ironflyer noun for that
// pillar. The grid degrades to one column on mobile and a 2x2 on desktop.
// FAQSection renders the Q&A list. Each item is a row in a single-column
// stack so the cadence feels like a reading lane rather than a card grid —
// readers scan top-to-bottom looking for their objection. Native <details>
// gives accordion behaviour without pulling in a heavy accordion lib.
// UseCaseGrid is the self-identification surface — two rows that let
// visitors filter what Ironflyer can build for their industry or role.
// Each chip is a Link to /solutions?filter=<tag> so future page work
// can surface tag-specific gallery views without changing the marketing
// home. Pattern lifted from Base44's "By industry" / "By role" panels.
// ComparisonTable is the head-to-head matrix. We name Base44, Lovable,
// Bolt by name because evasion looks weak to the comparison shopper.
// Ironflyer's column gets the lime accent + a sticky header on scroll
// (sticky CSS only — no JS) so the eye snaps to our line. The honest
// 'Coming Q3' entry for Visual Edits keeps the table credible.
function ComparisonTable() {
  const competitors = ['Ironflyer', 'Base44', 'Lovable', 'Bolt.new'] as const;
  return (
    <Section>
      <SectionHeader
        eyebrow="Side by side"
        title="What the finisher loop does that the others don’t."
      />
      <Box sx={{
        ...panelSx,
        overflow: 'auto',
        bgcolor: '#fffaf1',
      }}>
        <Box component="table" sx={{
          width: '100%',
          minWidth: 720,
          borderCollapse: 'collapse',
          fontSize: { xs: 14, md: 15 },
        }}>
          <Box component="thead" sx={{
            position: 'sticky', top: 0, zIndex: 1,
            bgcolor: '#fffaf1',
          }}>
            <Box component="tr">
              <Box component="th" sx={{
                textAlign: 'left', p: 2,
                borderBottom: '1px solid rgba(17,17,17,0.10)',
                color: '#5b554b', fontWeight: 800, letterSpacing: '0.06em',
                textTransform: 'uppercase', fontSize: 12,
              }}>
                Capability
              </Box>
              {competitors.map((name, i) => (
                <Box
                  key={name}
                  component="th"
                  sx={{
                    textAlign: 'center', p: 2,
                    borderBottom: '1px solid rgba(17,17,17,0.10)',
                    fontWeight: 900, fontSize: 14,
                    color: i === 0 ? '#0d0e0f' : '#3a352d',
                    bgcolor: i === 0 ? tokens.color.accent.lime : 'transparent',
                  }}
                >
                  {name}
                </Box>
              ))}
            </Box>
          </Box>
          <Box component="tbody">
            {comparisonRows.map((row, idx) => (
              <Box component="tr" key={row.label} sx={{
                bgcolor: idx % 2 === 0 ? 'transparent' : 'rgba(17,17,17,0.025)',
              }}>
                <Box component="td" sx={{
                  p: 2, borderBottom: '1px solid rgba(17,17,17,0.08)',
                  fontWeight: 700, color: '#111',
                }}>
                  {row.label}
                </Box>
                {row.values.map((v, i) => (
                  <Box
                    key={i}
                    component="td"
                    sx={{
                      p: 2, textAlign: 'center',
                      borderBottom: '1px solid rgba(17,17,17,0.08)',
                      fontWeight: i === 0 ? 800 : 600,
                      color: i === 0 ? '#0d0e0f' : '#3a352d',
                      bgcolor: i === 0 ? 'rgba(229,255,0,0.18)' : 'transparent',
                      fontFamily: v === '—' ? tokens.font.mono : tokens.font.family,
                    }}
                  >
                    {v}
                  </Box>
                ))}
              </Box>
            ))}
          </Box>
        </Box>
      </Box>
      <Typography variant="caption" sx={{
        display: 'block', mt: 1.5, color: '#5b554b',
        textAlign: 'center', maxWidth: 720, mx: 'auto',
      }}>
        Snapshot as of {new Date().toISOString().slice(0, 7)}. We update this
        table when competitors ship — open an issue if something shifted.
      </Typography>
    </Section>
  );
}

function UseCaseGrid() {
  const row = (title: string, items: { tag: string; label: string }[]) => (
    <Box>
      <Typography variant="overline" sx={{
        color: '#5b554b', fontWeight: 900, letterSpacing: '0.12em',
      }}>{title}</Typography>
      <Stack
        direction="row" spacing={1} flexWrap="wrap"
        sx={{ mt: 1.2, rowGap: 1 }}
      >
        {items.map((it) => (
          <Chip
            key={it.tag}
            label={it.label}
            component={Link}
            href={`/solutions?filter=${encodeURIComponent(it.tag)}`}
            clickable
            sx={{
              bgcolor: '#fffaf1', color: '#111',
              border: '1px solid rgba(17,17,17,0.10)',
              fontWeight: 800, px: 0.3,
              '& .MuiChip-label': { px: 1.4 },
              '&:hover': { bgcolor: tokens.color.accent.lime, color: '#0d0e0f' },
              transition: `background-color ${tokens.motion.base} ${tokens.motion.curve}`,
            }}
          />
        ))}
      </Stack>
    </Box>
  );
  return (
    <Section>
      <SectionHeader
        eyebrow="Use cases"
        title="Pick the row that looks like you."
      />
      <Stack spacing={{ xs: 3, md: 4 }}>
        {row('By industry', useCasesByIndustry)}
        {row('By role',     useCasesByRole)}
      </Stack>
    </Section>
  );
}

function FAQSection() {
  return (
    <Section>
      <SectionHeader
        eyebrow="FAQ"
        title="Common questions before you start the loop."
      />
      <Box sx={{ maxWidth: 920, mx: 'auto' }}>
        {faqs.map((item) => (
          <Box
            key={item.q}
            component="details"
            sx={{
              borderBottom: '1px solid rgba(17,17,17,0.12)',
              py: 2.2,
              cursor: 'pointer',
              '& > summary': { listStyle: 'none', outline: 'none' },
              '& > summary::-webkit-details-marker': { display: 'none' },
              '&[open] .faq-marker': { transform: 'rotate(45deg)' },
            }}>
            <Box component="summary" sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 2,
            }}>
              <Typography variant="h5" sx={{
                fontSize: { xs: '1.15rem', md: '1.4rem' },
                fontWeight: 800,
                letterSpacing: 0,
                color: '#111',
              }}>
                {item.q}
              </Typography>
              <Box className="faq-marker" sx={{
                width: 22, height: 22, flexShrink: 0,
                position: 'relative',
                transition: `transform ${tokens.motion.base} ${tokens.motion.curve}`,
                '&::before, &::after': {
                  content: '""',
                  position: 'absolute',
                  background: '#111',
                  left: '50%', top: '50%',
                  transform: 'translate(-50%, -50%)',
                },
                '&::before': { width: 14, height: 2 },
                '&::after':  { width: 2, height: 14 },
              }} />
            </Box>
            <Typography sx={{
              mt: 1.5,
              color: '#3a352d',
              fontSize: { xs: '0.95rem', md: '1.02rem' },
              lineHeight: 1.6,
              maxWidth: 780,
            }}>
              {item.a}
            </Typography>
          </Box>
        ))}
      </Box>
    </Section>
  );
}

function CapabilityTour() {
  return (
    <Section>
      <SectionHeader
        eyebrow="What ships with the loop"
        title="From prompt to production, gated end-to-end."
      />
      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)' },
        gap: { xs: 1.5, md: 2 },
      }}>
        {capabilityTourCards.map((card) => (
          <Box key={card.eyebrow} sx={{
            ...panelSx,
            p: { xs: 2.6, md: 3.4 },
            display: 'flex',
            flexDirection: 'column',
            gap: 2,
            minHeight: { xs: 240, md: 300 },
            position: 'relative',
            transition: `transform ${tokens.motion.base} ${tokens.motion.curve}`,
            '&:hover': { transform: 'translateY(-2px)' },
          }}>
            <Typography variant="overline" sx={{
              color: '#5b554b',
              fontWeight: 900,
              letterSpacing: '0.12em',
            }}>
              {card.eyebrow}
            </Typography>
            <Typography variant="h3" sx={{
              fontSize: { xs: '1.85rem', md: '2.35rem' },
              lineHeight: 1.02,
              letterSpacing: 0,
              maxWidth: 520,
            }}>
              {card.title}
            </Typography>
            <Typography variant="body1" sx={{
              color: '#3a352d',
              fontWeight: 500,
              maxWidth: 520,
              flex: 1,
            }}>
              {card.text}
            </Typography>
            <Chip
              label={card.chip}
              size="small"
              sx={{
                alignSelf: 'flex-start',
                bgcolor: '#111',
                color: tokens.color.accent.lime,
                borderRadius: 999,
                fontWeight: 900,
                px: 0.4,
                '& .MuiChip-label': { px: 1.2 },
              }}
            />
          </Box>
        ))}
      </Box>
    </Section>
  );
}

function MeetSection() {
  return (
    <Section>
      <SectionHeader eyebrow="Meet Ironflyer" title="Start with an idea. Leave with a product shape." />
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '0.95fr 1.05fr' }, gap: { xs: 3, md: 6 }, alignItems: 'center' }}>
        <Box sx={{ height: { xs: 300, md: 430 }, borderRadius: { xs: 3, md: 4 }, bgcolor: '#eee8dc', overflow: 'hidden' }}>
          <Box component="img" src={`${imageBase}/hooked.png`} alt="" sx={{ width: '100%', height: '100%', objectFit: 'cover', display: 'block' }} />
        </Box>
        <Stack spacing={{ xs: 2.5, md: 4 }}>
          {steps.map((step, index) => (
            <Box key={step.label} sx={{ opacity: index === 0 ? 1 : 0.48 }}>
              <Typography variant="h3" sx={{ fontSize: { xs: '2rem', md: '2.55rem' }, lineHeight: 1, letterSpacing: 0 }}>
                {step.label}
              </Typography>
              <Typography variant="h6" sx={{ mt: 0.75, color: '#4f4b43', fontWeight: 500, maxWidth: 560 }}>
                {step.text}
              </Typography>
            </Box>
          ))}
        </Stack>
      </Box>
    </Section>
  );
}

function ProductShowcase() {
  return (
    <Section>
      <SectionHeader eyebrow="Product" title="AI product tools, built like a real studio." />
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: { xs: 2.5, md: 3 } }}>
        {productTiles.map((tile, index) => (
          <Box key={tile.title} sx={{
            ...panelSx,
            gridColumn: { md: index === 0 ? '1 / -1' : 'auto' },
            p: { xs: 2, md: 2 },
          }}>
            <Box sx={{
              height: { xs: 290, md: index === 0 ? 460 : 330 },
              borderRadius: { xs: 2.5, md: 4 },
              overflow: 'hidden',
              bgcolor: '#d8cfbd',
            }}>
              <Box component="img" src={tile.image} alt="" sx={{
                width: '100%',
                height: '100%',
                objectFit: 'cover',
                display: 'block',
                transition: `transform ${tokens.motion.slow} ${tokens.motion.curve}`,
              }} />
            </Box>
            <Box sx={{ px: { xs: 0.5, md: 1 }, pt: 2, pb: 1.5, display: 'grid', gridTemplateColumns: { xs: '1fr', md: index === 0 ? '0.9fr 1fr auto' : '1fr auto' }, gap: 2, alignItems: 'start' }}>
              <Box>
                <Typography variant="overline" sx={{ color: '#111', fontWeight: 900 }}>IRONFLYER</Typography>
                <Typography variant="h3" sx={{ fontSize: { xs: '2.35rem', md: index === 0 ? '3.2rem' : '2.55rem' }, lineHeight: 0.92 }}>{tile.title}</Typography>
              </Box>
              <Typography variant="body1" sx={{ color: '#28251f', fontWeight: 600, maxWidth: index === 0 ? 520 : 440 }}>
                {tile.text}
              </Typography>
              <Chip label={index === 0 ? 'Core' : index === 1 ? 'Agents' : 'Runtime'} sx={{ width: 'fit-content', bgcolor: '#8e897d', color: '#fff', borderRadius: 999, fontWeight: 900 }} />
            </Box>
          </Box>
        ))}
      </Box>
    </Section>
  );
}

function RevenueEngineSection() {
  return (
    <Section>
      <Box sx={{ ...panelSx, p: { xs: 2.5, md: 4 }, bgcolor: '#111', color: tokens.color.bg.alabaster }}>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '0.9fr 1.1fr' }, gap: { xs: 3, md: 5 }, alignItems: 'center' }}>
          <Box>
            <Typography variant="overline" sx={{ color: tokens.color.accent.lime, fontWeight: 900 }}>Revenue architecture</Typography>
            <Typography variant="h2" sx={{ mt: 1, fontSize: { xs: '2.8rem', md: '5rem' }, lineHeight: 0.9, letterSpacing: 0 }}>
              Not just generated code. A product people can buy.
            </Typography>
            <Typography variant="h6" sx={{ mt: 2, color: '#d6cfbf', fontWeight: 500, maxWidth: 640 }}>
              The product needs a clean path from first prompt to paid workspace: visible usage, clear upgrade moments, reusable templates, and enterprise controls.
            </Typography>
          </Box>
          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(3, 1fr)' }, gap: 1.2 }}>
            {revenuePillars.map((item, index) => (
              <Box key={item.title} sx={{ minHeight: 230, p: 2, borderRadius: 2, bgcolor: index === 0 ? tokens.color.accent.lime : 'rgba(244,240,232,0.08)', color: index === 0 ? '#111' : tokens.color.bg.alabaster, display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
                <Typography variant="h4" sx={{ fontSize: { xs: '2rem', md: '2.45rem' }, lineHeight: 0.95 }}>{index + 1}</Typography>
                <Box>
                  <Typography variant="h6" sx={{ fontWeight: 900 }}>{item.title}</Typography>
                  <Typography variant="body2" sx={{ mt: 1, color: index === 0 ? '#24210d' : '#cfc7b8', fontWeight: 600 }}>{item.text}</Typography>
                </Box>
              </Box>
            ))}
          </Box>
        </Box>
      </Box>
    </Section>
  );
}

function BlueprintSection({ standalone = false }: { standalone?: boolean }) {
  return (
    <Section>
      <SectionHeader
        eyebrow="Templates"
        title={standalone ? 'Choose a blueprint' : 'Discover templates'}
        action={<Button component={Link} href="/templates" variant="outlined" endIcon={<ArrowForward />} sx={{ color: '#111', borderColor: 'rgba(17,17,17,0.25)' }}>View all</Button>}
      />
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', lg: 'repeat(4, 1fr)' }, gap: { xs: 2, md: 3 } }}>
        {blueprints.map((item) => (
          <Link key={item.title} href="/app" style={{ color: 'inherit', textDecoration: 'none' }}>
            <Box sx={{
              ...panelSx,
              p: 1,
              minHeight: 360,
              transition: `transform ${tokens.motion.base} ${tokens.motion.curve}`,
              '&:hover': { transform: 'translateY(-4px)' },
              '&:hover img': { transform: 'scale(1.04)' },
            }}>
              <Box sx={{ height: 220, overflow: 'hidden', borderRadius: { xs: 2, md: 3 }, bgcolor: '#d5cab7' }}>
                <Box component="img" src={item.image} alt="" sx={{
                  width: '100%',
                  height: '100%',
                  objectFit: 'cover',
                  transition: `transform ${tokens.motion.slow} ${tokens.motion.curve}`,
                  display: 'block',
                }} />
              </Box>
              <Box sx={{ p: 1.5 }}>
                <Typography variant="h5" sx={{ letterSpacing: 0, fontWeight: 900 }}>{item.title}</Typography>
                <Typography variant="body2" sx={{ color: '#4f4b43', fontWeight: 600 }}>{item.label}</Typography>
                <Button sx={{ mt: 1.5, px: 0, color: tokens.color.accent.lime, fontWeight: 900 }} endIcon={<ArrowForward />}>Use this blueprint</Button>
              </Box>
            </Box>
          </Link>
        ))}
      </Box>
    </Section>
  );
}

function NumbersSection() {
  const stats = [
    ['7', 'release gates'],
    ['8', 'agent roles'],
    ['1', 'shared workspace'],
  ];
  return (
    <Section>
      <Box>
        <Typography variant="h2" sx={{ mb: 2, letterSpacing: 0, fontSize: { xs: '3rem', md: '4.8rem' }, lineHeight: 0.92 }}>Ironflyer</Typography>
        <Typography variant="h6" sx={{ mb: 4, color: '#302c25', fontWeight: 600 }}>Millions of builders are already turning ideas into reality</Typography>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 1.5 }}>
          {stats.map(([value, label]) => (
            <Box key={label} sx={{ bgcolor: '#f8f4ec', borderRadius: { xs: 3, md: 4 }, minHeight: 260, p: { xs: 3, md: 4 }, display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
              <Typography variant="h1" sx={{ fontSize: { xs: '5rem', md: '6rem' }, lineHeight: 0.9, letterSpacing: 0, color: '#111' }}>
                {value}
              </Typography>
              <Typography variant="body1" sx={{ color: '#111', fontWeight: 700 }}>{label}</Typography>
            </Box>
          ))}
        </Box>
      </Box>
    </Section>
  );
}

function PricingSection() {
  return (
    <Section>
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '0.9fr 1.1fr' }, gap: 1.5, mb: 2 }}>
        <Box sx={{ ...panelSx, p: 3, bgcolor: '#111', color: tokens.color.bg.alabaster }}>
          <Typography variant="overline" sx={{ color: tokens.color.accent.lime }}>Pricing strategy</Typography>
          <Typography variant="h3" sx={{ mt: 1, fontSize: { xs: '2.25rem', md: '3.2rem' }, lineHeight: 0.95 }}>
            Price around outcomes, protect trust with visible spend.
          </Typography>
          <Typography variant="body1" sx={{ mt: 2, color: '#d6cfbf', fontWeight: 600 }}>
            The market is already trained on credits, team seats, private projects, and enterprise controls. Ironflyer should make those controls obvious before asking for payment.
          </Typography>
        </Box>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(3, 1fr)' }, gap: 1.5 }}>
          {pricingAssumptions.map((item) => (
            <Box key={item.label} sx={{ ...panelSx, p: 2.2, minHeight: 190, display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
              <CheckCircle sx={{ color: tokens.color.accent.lime }} />
              <Box>
                <Typography variant="h6" sx={{ fontWeight: 900 }}>{item.label}</Typography>
                <Typography variant="body2" sx={{ mt: 1, color: '#4f4b43', fontWeight: 600 }}>{item.text}</Typography>
              </Box>
            </Box>
          ))}
        </Box>
      </Box>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(4, 1fr)' }, gap: 1.5 }}>
        {plans.map((plan) => {
          const tier = plan.name.toLowerCase() as 'free' | 'pro' | 'team' | 'enterprise';
          return (
            <Box key={plan.name} sx={{ ...panelSx, p: 3, minHeight: 420, display: 'flex', flexDirection: 'column', bgcolor: plan.name === 'Team' ? '#111' : '#e8dfce', color: plan.name === 'Team' ? tokens.color.bg.alabaster : '#111' }}>
              <Chip label={plan.badge} sx={{ width: 'fit-content', borderRadius: 1, bgcolor: plan.name === 'Team' ? 'rgba(229,255,0,0.18)' : '#d9cfbd', color: plan.name === 'Team' ? tokens.color.accent.lime : '#111', fontWeight: 900 }} />
              <Typography variant="overline" sx={{ mt: 2, color: plan.name === 'Team' ? tokens.color.accent.lime : '#5b554b' }}>{plan.name}</Typography>
              <Stack direction="row" alignItems="baseline" spacing={0.4} sx={{ mt: 1 }}>
                <Typography variant="h2" sx={{ letterSpacing: 0 }}>{plan.price}</Typography>
                {plan.period && <Typography variant="body2" sx={{ color: plan.name === 'Team' ? '#cfc7b8' : '#4f4b43', fontWeight: 800 }}>{plan.period}</Typography>}
              </Stack>
              <Typography variant="body2" sx={{ mt: 2, flex: 1, color: plan.name === 'Team' ? '#cfc7b8' : '#4f4b43', fontWeight: 600 }}>{plan.text}</Typography>
              <Stack spacing={1} sx={{ my: 2 }}>
                {plan.features.map((feature) => (
                  <Stack key={feature} direction="row" spacing={1} alignItems="center">
                    <CheckCircle sx={{ fontSize: 17, color: tokens.color.accent.lime }} />
                    <Typography variant="caption" sx={{ color: plan.name === 'Team' ? '#eee7db' : '#28251f', fontWeight: 800 }}>{feature}</Typography>
                  </Stack>
                ))}
              </Stack>
              {tier === 'free' ? (
                <Button component={Link} href="/app" variant="contained" sx={{ mt: 3 }}>
                  {plan.cta}
                </Button>
              ) : (
                <Box sx={{ mt: 3, display: 'flex', flexDirection: 'column' }}>
                  <UpgradeButton tier={tier} label={plan.cta} />
                </Box>
              )}
            </Box>
          );
        })}
      </Box>
    </Section>
  );
}

function TemplatesLibrarySection() {
  return (
    <Box component="main" sx={{ bgcolor: tokens.color.bg.alabaster, color: tokens.color.text.inverse, minHeight: '100vh', pt: { xs: 5, md: 7 }, pb: { xs: 6, md: 10 } }}>
      <Container maxWidth="xl">
        <Stack spacing={5}>
          <Box sx={{ maxWidth: 760 }}>
            <Typography component="h1" variant="h1" sx={{
              fontSize: { xs: '3rem', md: '5rem' },
              lineHeight: 0.88,
              letterSpacing: 0,
            }}>
              Website & App Templates Built With AI
            </Typography>
            <Typography variant="h5" sx={{ mt: 2, color: '#5d5d5d', fontWeight: 500 }}>
              Production-ready blueprints shaped for the Ironflyer finisher loop.
            </Typography>
          </Box>

          <Box sx={{
            display: 'flex',
            gap: { xs: 2, md: 4 },
            overflowX: 'auto',
            pb: 1,
            borderBottom: '1px solid rgba(25,25,25,0.12)',
          }}>
            {templateCategories.map((category, index) => (
              <Stack
                key={category.label}
                alignItems="center"
                spacing={1}
                sx={{
                  minWidth: 104,
                  color: index === 0 ? '#111' : '#666',
                  pb: 1.5,
                  borderBottom: index === 0 ? '2px solid #111' : '2px solid transparent',
                }}
              >
                {category.icon}
                <Typography variant="caption" sx={{ fontWeight: 800, whiteSpace: 'nowrap' }}>
                  {category.label}
                </Typography>
              </Stack>
            ))}
          </Box>

          <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: { xs: 3, md: 5 } }}>
            {templateLibrary.map((item) => (
              <Link key={item.title} href="/app" style={{ color: 'inherit', textDecoration: 'none' }}>
                <Box sx={{ '&:hover img': { transform: 'scale(1.025)' } }}>
                  <Box sx={{
                    height: { xs: 240, md: 280 },
                    borderRadius: { xs: 2.5, md: 4 },
                    overflow: 'hidden',
                    bgcolor: '#ddd2bf',
                  }}>
                    <Box component="img" src={item.image} alt="" sx={{
                      width: '100%',
                      height: '100%',
                      objectFit: 'cover',
                      display: 'block',
                      transition: `transform ${tokens.motion.slow} ${tokens.motion.curve}`,
                    }} />
                  </Box>
                  <Stack direction="row" justifyContent="space-between" spacing={2} sx={{ mt: 1.5 }}>
                    <Box sx={{ minWidth: 0 }}>
                      <Typography variant="h6" sx={{ fontWeight: 900 }} noWrap>{item.title}</Typography>
                      <Typography variant="body2" sx={{ color: '#5d5d5d' }}>{item.desc}</Typography>
                    </Box>
                    <Chip label={item.tag} size="small" sx={{ bgcolor: '#ece6db', borderRadius: 1, fontWeight: 800 }} />
                  </Stack>
                </Box>
              </Link>
            ))}
          </Box>
        </Stack>
      </Container>
    </Box>
  );
}

function GateBand() {
  return (
    <Section>
      <Box sx={{ ...panelSx, p: { xs: 3, md: 5 }, bgcolor: '#111', color: tokens.color.bg.alabaster }}>
        <SectionHeader eyebrow="Finisher gates" title="Every release has to pass the same checkpoints." compact />
        <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap">
          {gates.map((gate, index) => (
            <Chip
              key={gate}
              icon={index < 5 ? <CheckCircle /> : <Lock />}
              label={gate}
              sx={{
                borderRadius: 1,
                bgcolor: index < 5 ? 'rgba(229,255,0,0.14)' : 'rgba(103,29,252,0.16)',
                color: tokens.color.bg.alabaster,
                border: '1px solid rgba(244,240,232,0.2)',
                p: 2.2,
              }}
            />
          ))}
        </Stack>
      </Box>
    </Section>
  );
}

function CardGrid({ items, icon }: { items: { title: string; text: string }[]; icon: React.ReactNode }) {
  return (
    <Section>
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(4, 1fr)' }, gap: 1.5 }}>
        {items.map((item) => (
          <Box key={item.title} sx={{ ...panelSx, p: 3, minHeight: 250 }}>
            <Box sx={{ color: '#111', mb: 5 }}>{icon}</Box>
            <Typography variant="h5" sx={{ letterSpacing: 0 }}>{item.title}</Typography>
            <Typography variant="body2" sx={{ mt: 1.5, color: '#4f4b43', fontWeight: 600 }}>{item.text}</Typography>
          </Box>
        ))}
      </Box>
    </Section>
  );
}

function EnterpriseLeadSection() {
  return (
    <Section>
      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '0.8fr 1.2fr' }, gap: { xs: 2, md: 3 }, alignItems: 'stretch' }}>
        <Box sx={{ ...panelSx, p: { xs: 3, md: 4 }, display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
          <Box>
            <Typography variant="overline" sx={{ color: tokens.color.accent.lime, fontWeight: 900 }}>Qualified demand</Typography>
            <Typography variant="h2" sx={{ mt: 1, fontSize: { xs: '2.8rem', md: '4.4rem' }, lineHeight: 0.9 }}>
              Sell the controlled build system, not another chat box.
            </Typography>
            <Typography variant="body1" sx={{ mt: 2, color: '#4f4b43', fontWeight: 700 }}>
              Enterprise buyers need identity, auditability, budget controls, private connectors, and a clear migration path from prototype to production.
            </Typography>
          </Box>
          <Stack spacing={1} sx={{ mt: 3 }}>
            {['SSO and SCIM readiness', 'Private repo and connector mapping', 'Budget guardrails by team', 'Onboarding and implementation plan'].map((item) => (
              <Stack key={item} direction="row" spacing={1} alignItems="center">
                <CheckCircle sx={{ fontSize: 18, color: tokens.color.accent.lime }} />
                <Typography variant="body2" sx={{ fontWeight: 900 }}>{item}</Typography>
              </Stack>
            ))}
          </Stack>
        </Box>
        <EnterpriseLeadForm />
      </Box>
    </Section>
  );
}

function PageHero({ eyebrow, title, text, image }: { eyebrow: string; title: string; text: string; image: string }) {
  return (
    <Box sx={{
      bgcolor: tokens.color.bg.alabaster,
      pt: { xs: 4, md: 6 },
    }}>
      <Container maxWidth="xl">
        <Box sx={{ borderRadius: { xs: 3, md: 5 }, overflow: 'hidden', minHeight: { xs: 430, md: 560 }, display: 'flex', alignItems: 'center', justifyContent: 'center', textAlign: 'center', px: { xs: 3, md: 8 }, backgroundImage: `linear-gradient(180deg, rgba(13,14,15,0.1), rgba(13,14,15,0.45)), url(${image})`, backgroundSize: 'cover', backgroundPosition: 'center' }}>
          <Stack spacing={2.5} alignItems="center" sx={{ maxWidth: 960 }}>
            <Typography variant="overline" sx={{ color: tokens.color.accent.lime, fontWeight: 900 }}>{eyebrow}</Typography>
            <Typography component="h1" variant="h1" sx={{
              fontSize: { xs: '3rem', md: '6.1rem' },
              lineHeight: 0.86,
              letterSpacing: 0,
              color: tokens.color.bg.alabaster,
              textWrap: 'balance',
            }}>
              {title}
            </Typography>
            <Typography variant="h5" sx={{ maxWidth: 680, color: tokens.color.bg.alabaster, fontWeight: 700, fontSize: { xs: '1.05rem', md: '1.35rem' } }}>{text}</Typography>
          </Stack>
        </Box>
      </Container>
    </Box>
  );
}

function FinalCta({ primary = 'Create your workspace', secondary = 'Explore product' }: { primary?: string; secondary?: string }) {
  return (
    <Section>
      <Box sx={{ ...panelSx, p: { xs: 3, md: 6 }, bgcolor: '#111', color: tokens.color.bg.alabaster, display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.2fr 0.8fr' }, gap: 3 }}>
        <Box>
          <Typography variant="h2" sx={{ letterSpacing: 0, fontSize: { xs: '2.7rem', md: '4.1rem' }, lineHeight: 0.95 }}>Ready to build?</Typography>
          <Typography variant="h6" sx={{ mt: 2, fontWeight: 500, color: '#d6cfbf' }}>
            Bring an idea. Ironflyer will push it through the same product, engineering, and release gates every time.
          </Typography>
        </Box>
        <Stack direction={{ xs: 'column', sm: 'row', md: 'column' }} spacing={1.5} justifyContent="center">
          <Button component={Link} href="/app" variant="contained" size="large" endIcon={<RocketLaunch />} sx={{ py: 1.5 }}>{primary}</Button>
          <Button component={Link} href="/product" variant="outlined" size="large" endIcon={<ArrowForward />} sx={{ color: tokens.color.bg.alabaster, borderColor: 'rgba(244,240,232,0.28)' }}>{secondary}</Button>
        </Stack>
      </Box>
    </Section>
  );
}

function Section({ children }: { children: React.ReactNode }) {
  return (
    <Box component="section" sx={{ py: { xs: 6, md: 10 }, bgcolor: tokens.color.bg.alabaster }}>
      <Container maxWidth="xl">{children}</Container>
    </Box>
  );
}

function SectionHeader({
  eyebrow, title, action, compact = false,
}: {
  eyebrow: string;
  title: string;
  action?: React.ReactNode;
  compact?: boolean;
}) {
  return (
    <Stack
      direction={{ xs: 'column', md: 'row' }}
      justifyContent="space-between"
      alignItems={{ xs: 'flex-start', md: 'flex-end' }}
      spacing={2}
      sx={{ mb: compact ? 3 : 4 }}
    >
      <Box>
        <Typography variant="overline" sx={{ color: tokens.color.accent.lime, fontWeight: 900 }}>{eyebrow}</Typography>
        <Typography variant="h2" sx={{
          mt: 1,
          maxWidth: 980,
          fontSize: compact ? { xs: '2.65rem', md: '5.2rem' } : { xs: '2.85rem', md: '5.9rem' },
          lineHeight: 0.88,
          letterSpacing: 0,
          textWrap: 'balance',
        }}>
          {title}
        </Typography>
      </Box>
      {action}
    </Stack>
  );
}

function SiteFooter() {
  return (
    <Box component="footer" sx={{ bgcolor: '#0d0e0f', color: tokens.color.bg.alabaster, mt: 0 }}>
      <Container maxWidth="xl" sx={{ py: { xs: 7, md: 9 } }}>
        <Typography variant="h2" sx={{ fontSize: { xs: '3rem', md: '4.8rem' }, lineHeight: 0.92, mb: { xs: 6, md: 8 } }}>
          Start making incredible software
        </Typography>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.2fr repeat(4, 1fr)' }, gap: 4 }}>
          <Box>
            <Stack direction="row" alignItems="center" spacing={1.5}>
              <Box sx={{ width: 28, height: 28, borderRadius: 1, bgcolor: tokens.color.accent.lime }} />
              <Typography variant="h6" sx={{ fontFamily: tokens.font.display, fontWeight: 400, letterSpacing: 0 }}>Ironflyer</Typography>
            </Stack>
            <Typography variant="body2" sx={{ mt: 2, maxWidth: 300, color: '#8d887e' }}>
              AI product finishing for teams that want working software with fewer loose ends.
            </Typography>
          </Box>
          {footerGroups.map((group) => (
            <Box key={group.title}>
              <Typography variant="overline" sx={{ color: '#8d887e' }}>{group.title}</Typography>
              <Stack spacing={1.2} sx={{ mt: 1.5 }}>
                {group.links.map((link) => (
                  <Typography key={link} variant="body2" sx={{ color: tokens.color.bg.alabaster, fontWeight: 700 }}>
                    {link}
                  </Typography>
                ))}
              </Stack>
            </Box>
          ))}
        </Box>
        <Divider sx={{ my: 4, borderColor: 'rgba(244,240,232,0.12)' }} />
        <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" spacing={1}>
          <Typography variant="caption" sx={{ color: '#8d887e' }}>©2026 Ironflyer. All rights reserved.</Typography>
          <Stack direction="row" spacing={2} sx={{ color: '#8d887e' }}>
            <Tune fontSize="small" />
            <Hub fontSize="small" />
            <Code fontSize="small" />
            <Inventory2 fontSize="small" />
          </Stack>
        </Stack>
      </Container>
    </Box>
  );
}
