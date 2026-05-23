import Link from 'next/link';
import { Suspense } from 'react';
import {
  ArrowForward, AutoAwesome, Bolt, Build, CheckCircle, Code, FormatQuote,
  Insights, Lock, RocketLaunch, Settings, Shield, Speed, Terminal,
  TrendingUp, VerifiedUser, Visibility, Whatshot, WorkOutline,
} from '@mui/icons-material';
import {
  Box, Button, Chip, Container, Stack, Typography,
} from '@mui/material';
import { tokens } from '../../../packages/design-tokens';
import { BillingStatusBanner } from './billing-status-banner';
import { EnterpriseLeadForm } from './enterprise-lead-form';
import { HeroQuickStarts } from './hero-quick-starts';
import { MarketingShellClient } from './marketing-shell';
import { PricingCalculator } from './pricing-calculator';
import { PromptBox } from './prompt-box';
import { TemplatesGrid } from './templates-grid';
import { UpgradeButton } from './upgrade-button';

// -- Tokens / shared sx --------------------------------------------------

const ALABASTER = tokens.color.bg.alabaster;
const INK = '#0d0e0f';
const LIME = tokens.color.accent.lime;
const MUTED = '#5b554b';

const panelSx = {
  borderRadius: 1,
  backgroundColor: '#ece5d4',
  overflow: 'hidden',
  color: INK,
} as const;

// -- Static content ------------------------------------------------------

const heroQuickStarts = [
  { label: 'SaaS dashboard',  prompt: 'Build a production-ready SaaS app with auth, teams, Stripe billing, usage analytics, admin settings, onboarding flow, and a one-click deploy gate.' },
  { label: 'Internal tool',   prompt: 'Build an internal operations tool with role-based approvals, audit history, dense table UI, CSV export, and Postgres for the data model.' },
  { label: 'Marketplace',     prompt: 'Build a two-sided marketplace with listings, search filters, messaging, Stripe Connect escrow payouts, and a trust-score profile page.' },
  { label: 'AI chatbot',      prompt: 'Build an AI customer-support chatbot with retrieval over uploaded docs, session memory, hand-off to human, and an admin analytics dashboard.' },
  { label: 'Launch site',     prompt: 'Build a product launch site with hero, waitlist form, pricing, FAQ, social proof, and analytics events wired to the dataLayer.' },
  { label: 'Client portal',   prompt: 'Build a client portal with auth, document uploads, project status, threaded messaging, role-aware access, and an admin console.' },
];

const gateExamples = [
  {
    name: 'Spec',
    icon: <WorkOutline />,
    title: 'Spec gate',
    output: `✓ user_stories: 14
✓ data_model: 6 entities, 11 relations
✓ acceptance_criteria: complete
✗ open_questions: 2
  - payout currency (USD only or multi?)
  - admin override on refunds?`,
    caption: 'Refuses to advance until the open product questions get answered — no silent assumption shipping.',
  },
  {
    name: 'Code',
    icon: <Code />,
    title: 'Code gate',
    output: `✓ go build       : clean
✓ go vet         : clean
✓ npx tsc        : 0 errors
✗ golangci-lint  : 3 issues
  internal/billing/ledger.go:142 — errcheck
  apps/web/app/projects/page.tsx:88 — react-hooks/exhaustive-deps
  apps/web/lib/api.ts:301 — no-unused-vars`,
    caption: 'Patches that don’t build never land. The agent re-plans, patches, and re-runs until lint is green.',
  },
  {
    name: 'Security',
    icon: <Shield />,
    title: 'Security gate',
    output: `✓ secret_scan       : no credentials in patch
✓ dep_audit         : 0 high / 0 critical
✗ owasp_a02         : 1 finding
  apps/web/app/api/route.ts — unsafe HTML render of user input
  → suggest: wrap with sanitizeHtml() (DOMPurify) before dangerouslySetInnerHTML`,
    caption: 'OWASP-shaped checks run on the diff, not just the snapshot. The fix is in the same turn.',
  },
  {
    name: 'Budget',
    icon: <Insights />,
    title: 'Budget gate',
    output: `subscription   : $20.00 / month
provider cost  : $14.62 / month  (73% of cap)
margin         : $5.38 / month  ✓ positive
top models     :
  claude-haiku-4.5    $8.10  spec + ux
  claude-sonnet-4.6   $5.20  code + tests
  gpt-4o-mini         $1.32  cheap re-runs`,
    caption: 'Every provider call charges the ledger. When the cap nears, the router downgrades; nothing surprises the bill.',
  },
];

const capabilityCards = [
  {
    eyebrow: '01 · Governed generation',
    title: 'The app does not move forward until the gate says yes.',
    text: 'Spec, UX, Architecture, Code, Lint, Tests, Security, Budget, Deploy. Each gate produces a readable verdict, blocks bad work, and gives the agent a repair target.',
    chip: 'Finisher loop',
    icon: <CheckCircle />,
  },
  {
    eyebrow: '02 · Reviewable changes',
    title: 'Every AI edit becomes a patch, not a mystery write.',
    text: 'The agent proposes a patch, the engine validates it, the UI previews the diff, and the workspace applies it only after the lifecycle accepts the change.',
    chip: 'Patch lifecycle',
    icon: <Terminal />,
  },
  {
    eyebrow: '03 · Real runtime',
    title: 'A Linux workspace your engineers can actually inspect.',
    text: 'Per-user Docker sandboxes expose files, terminal, preview, and execution logs. Open the same project from the browser, VSCode, or the runtime API.',
    chip: 'Docker workspace',
    icon: <Insights />,
  },
  {
    eyebrow: '04 · Cost discipline',
    title: 'The model bill is visible before it becomes a problem.',
    text: 'Every provider call lands in the ledger. Capability routing chooses the cheapest model that can pass the gate, then downgrades or pauses when the cap gets close.',
    chip: 'Budget ledger',
    icon: <AutoAwesome />,
  },
];

const comparisonRows: { label: string; values: [string, string, string, string] }[] = [
  { label: 'Enforced finisher gates (refuses to ship if any fail)',  values: ['9 gates',         '—',           '—',           '—'] },
  { label: 'Budget gate blocks deploy on overspend',                  values: ['✓ Enforced',      '—',           '—',           '—'] },
  { label: 'Live $-burn vs cost cap (no credit traps)',              values: ['✓ Live ledger',   'Credit packs', 'Credit packs', 'Token bucket'] },
  { label: 'Per-user Linux sandbox + real terminal',                 values: ['✓ Docker',        'Hosted',      'Hosted',      'WebContainer'] },
  { label: 'Multi-provider routing (Anthropic + OpenAI + on-device)', values: ['✓ Capability-tagged', '—',     'Internal',    'Frontier coder'] },
  { label: 'Multi-provider routing (Anthropic + OpenAI + Gemini)',    values: ['✓ 3 providers',  'Hidden',      '✓',           'Switchable'] },
  { label: 'OpenTelemetry traces out of the box',                     values: ['✓ OTLP',         '—',           '—',           '—'] },
  { label: 'Inline AI image generation in patches',                   values: ['✓ OpenAI Images', '✓',          '—',           '—'] },
  { label: 'Project dependency graph (imports/exports)',              values: ['✓ API',          '—',           '—',           '—'] },
  { label: 'Effort dial (Lite / Economy / Power)',                    values: ['✓',              '—',           '—',           '—'] },
  { label: 'Bring-your-own cloud via Helm chart',                     values: ['✓ Shipped',       '—',           '—',           '—'] },
  { label: 'VSCode extension (native client)',                        values: ['✓',              '—',           '—',           '—'] },
  { label: 'GitHub bi-directional push',                              values: ['✓',              '✓',           '✓',           '✓'] },
  { label: 'Image-attached prompts (design refs in chat)',            values: ['✓',              '✓',           '✓',           '✓'] },
  { label: 'Pixel-perfect visual diff gate',                          values: ['✓ Blocking',     '—',           '—',           '—'] },
  { label: 'Persistent project memory (4 dimensions)',                values: ['✓ Auto-captured', '—',          '—',           '—'] },
  { label: 'Hash-chained audit log',                                  values: ['✓ SHA-256 chain', '—',          '—',           '—'] },
  { label: 'MCP integration (server + client)',                       values: ['✓ Both directions', '—',        '—',           '✓ partial'] },
  { label: 'Database + auth scaffolded into generated apps',         values: ['✓ 6 domain packs', '✓',          '✓ Supabase',  '—'] },
  { label: 'HuggingFace open-model routing (Llama / Qwen / DeepSeek)', values: ['✓ 5 OSS models', '—',           '—',           '—'] },
  { label: 'Semantic memory retrieval (HF embeddings)',               values: ['✓ bge-small + cosine', '—',     '—',           '—'] },
  { label: 'Context7 docs lookup in every Coder call',                values: ['✓ Auto-registered', '—',        '—',           '—'] },
  { label: 'Persistent hash-chain audit (SurrealDB)',                 values: ['✓ Survives restart', '—',       '—',           '—'] },
  { label: 'Multi-agent voting on critical gates',                    values: ['✓ 3-way Critic', '—',           '—',           '—'] },
  { label: 'Post-patch reflection (accomplished / drift)',            values: ['✓ Memory-fed',   '—',           '—',           '—'] },
  { label: 'Native scaffolders for 12+ stacks (Rust / Java / .NET / Swift / Kotlin / Ruby / PHP / Python / Go / TS)', values: ['✓ 12 packs', 'Web only', 'Web only', 'Web only'] },
  { label: 'Monorepo subprojects (one repo, many services)',          values: ['✓ Native',       '—',           '—',           '—'] },
  { label: 'Migration manager (Drizzle / Prisma / Alembic / EF Core)', values: ['✓ Agent-driven', '—',          '—',           '—'] },
  { label: 'Production CI/CD scaffold (Actions + Argo CD + K8s)',     values: ['✓ Out of the box', '—',         '—',           '—'] },
  { label: 'Distributed locks + cross-pod rate limit (Redis)',        values: ['✓ Horizontal-scale ready', '—', '—',           '—'] },
];

const proofPoints = [
  {
    label: 'Blocking verdicts',
    value: '9 gates',
    text: 'Spec, UX, Architecture, Code, Lint, Tests, Security, Budget, and Deploy produce explicit pass/fail evidence before the project moves forward.',
  },
  {
    label: 'Change control',
    value: 'Patch-first',
    text: 'The agent proposes diffs through the patch engine. No direct mystery writes, and every change can be reviewed or rolled back.',
  },
  {
    label: 'Runtime ownership',
    value: 'Linux workspace',
    text: 'Files, terminal, preview ports, and execution logs live in a per-user workspace your team can inspect from web, API, or VSCode.',
  },
  {
    label: 'Stacks covered',
    value: '12 native',
    text: 'Rust, Go HTTP, Python FastAPI, Java Spring, Kotlin Android, Swift iOS, Rails, Laravel, .NET, Next.js, Phaser, Expo — auto-detected from the spec.',
  },
];

const useCasesByIndustry = [
  { tag: 'productivity',   label: 'Productivity' },
  { tag: 'education',      label: 'Education' },
  { tag: 'entertainment',  label: 'Entertainment' },
  { tag: 'health',         label: 'Health & wellness' },
  { tag: 'commerce',       label: 'E-commerce' },
  { tag: 'finance',        label: 'Finance' },
];

const useCasesByRole = [
  { tag: 'product',     label: 'Product Management' },
  { tag: 'operations',  label: 'Operations' },
  { tag: 'marketing',   label: 'Marketing & Sales' },
  { tag: 'hr',          label: 'HR & Recruitment' },
  { tag: 'engineering', label: 'Dev Productivity' },
  { tag: 'analytics',   label: 'Business Intelligence' },
];

// -- Brand copy bank -----------------------------------------------------

const brand = {
  heroBadge: 'Public beta · built for production-minded teams',
  heroTitle: ['Ship software ', 'that survives review.'],
  heroSubtitle: 'Describe the product. Ironflyer plans it, builds it, and pushes every change through Spec, UX, Architecture, Code, Lint, Tests, Security, Budget, and Deploy gates before anyone calls it finished.',
  heroCtaPrimary: 'Start building',
  heroCtaSecondary: 'See the loop',
  capabilityEyebrow: 'Why Ironflyer exists',
  capabilityTitle: 'AI builders are fast. Finishing is the hard part.',
  gatesEyebrow: 'Gate verdicts',
  gatesTitle: 'Every pass or block is written down.',
  socialEyebrow: 'Operating proof',
  socialTitle: 'Evidence beats promises.',
  compareEyebrow: 'Market position',
  compareTitle: 'Where Ironflyer draws the line.',
  ctaTitle: 'Bring the idea. Keep the standard.',
  ctaSubtitle: 'Start from a prompt and end with a repo, a running workspace, a budget ledger, and deploy artifacts your team can defend.',
};

// -----------------------------------------------------------------------
// Public exports (consumed by page.tsx files)
// -----------------------------------------------------------------------

export function MarketingShell({ children }: { children: React.ReactNode }) {
  return <MarketingShellClient>{children}</MarketingShellClient>;
}

export function MarketingHome() {
  return (
    <MarketingShell>
      <HeroSection />
      <LogoBand />
      <CapabilityTour />
      <GateExamples />
      <CloudIDEPreview />
      <BudgetSpotlight />
      <TestimonialsSection />
      <ComparisonTable />
      <UseCaseGrid />
      <FAQSection />
      <FinalCta />
    </MarketingShell>
  );
}

// -----------------------------------------------------------------------
// HERO
// -----------------------------------------------------------------------

function HeroSection() {
  return (
    <Box component="main" sx={{ bgcolor: ALABASTER, pt: { xs: 5, md: 8 }, pb: { xs: 8, md: 10 } }}>
      <Container maxWidth="xl">
        <Stack alignItems="center" spacing={{ xs: 3, md: 4 }} sx={{ textAlign: 'center' }}>
          <Chip
            label={brand.heroBadge}
            sx={{
              bgcolor: 'rgba(13,14,15,0.06)',
              color: INK,
              border: '1px solid rgba(13,14,15,0.1)',
              borderRadius: '999px',
              fontWeight: 800,
              fontSize: 12.5,
              px: 0.8,
              '& .MuiChip-label': { px: 1.6 },
            }}
          />
          <Typography
            component="h1"
            sx={{
              fontFamily: tokens.font.display,
              fontWeight: 400,
              fontSize: { xs: '3.4rem', sm: '5rem', md: '7.4rem' },
              lineHeight: 0.86,
              letterSpacing: 0,
              color: INK,
              maxWidth: 1100,
              textWrap: 'balance',
            }}
          >
            {brand.heroTitle[0]}
            <Box component="span" sx={{
              background: `linear-gradient(180deg, ${LIME} 0%, #b6db00 100%)`,
              WebkitBackgroundClip: 'text',
              WebkitTextFillColor: 'transparent',
              backgroundClip: 'text',
              position: 'relative',
              '&::after': {
                content: '""',
                position: 'absolute',
                left: 0, right: 0, bottom: '-0.06em',
                height: '0.14em',
                background: LIME,
                opacity: 0.18,
                borderRadius: 999,
              },
            }}>
              {brand.heroTitle[1]}
            </Box>
          </Typography>

          <Typography
            sx={{
              maxWidth: 760,
              color: '#3a352d',
              fontSize: { xs: '1.05rem', md: '1.32rem' },
              fontWeight: 500,
              lineHeight: 1.45,
            }}
          >
            {brand.heroSubtitle}
          </Typography>

          <Box sx={{
            width: '100%',
            maxWidth: 860,
            pt: { xs: 1, md: 2 },
          }}>
            <Box>
              <PromptBox
                size="hero"
                cta="Start the loop"
                placeholder="Describe the product you want to ship..."
              />
              <HeroQuickStarts items={heroQuickStarts} />
            </Box>
          </Box>

          <Stack direction="row" spacing={2.5} alignItems="center" sx={{ pt: 1.5, color: MUTED, flexWrap: 'wrap', justifyContent: 'center', rowGap: 1 }}>
            {[
              { icon: <CheckCircle sx={{ fontSize: 16 }} />, text: 'Blocking gates' },
              { icon: <Lock sx={{ fontSize: 16 }} />, text: 'Reviewable patches' },
              { icon: <Bolt sx={{ fontSize: 16 }} />, text: 'Live cost ledger' },
              { icon: <Speed sx={{ fontSize: 16 }} />, text: 'Deploy ownership' },
            ].map((item) => (
              <Stack key={item.text} direction="row" spacing={0.8} alignItems="center">
                <Box sx={{ color: '#6c8400', display: 'inline-flex' }}>{item.icon}</Box>
                <Typography variant="caption" sx={{ fontWeight: 700, fontSize: 12.5, color: INK }}>{item.text}</Typography>
              </Stack>
            ))}
          </Stack>
        </Stack>
      </Container>
    </Box>
  );
}

function LogoBand() {
  const labels = ['Spec review', 'Diff approval', 'Runtime logs', 'Security scan', 'Cost ledger', 'Deploy artifact'];
  return (
    <Box sx={{ bgcolor: ALABASTER, py: { xs: 4, md: 6 }, borderTop: '1px solid rgba(13,14,15,0.07)', borderBottom: '1px solid rgba(13,14,15,0.07)' }}>
      <Container maxWidth="lg">
        <Stack spacing={3} alignItems="center">
          <Typography variant="caption" sx={{ color: MUTED, fontWeight: 800, fontSize: 12, letterSpacing: '0.14em', textTransform: 'uppercase' }}>
            The review checklist is built into the loop
          </Typography>
          <Stack direction="row" spacing={{ xs: 3, md: 6 }} useFlexGap flexWrap="wrap" justifyContent="center" alignItems="center" sx={{ opacity: 0.78 }}>
            {labels.map((label) => (
              <Typography key={label} variant="h6" sx={{ fontFamily: tokens.font.display, fontWeight: 400, color: INK, fontSize: { xs: 16, md: 20 }, letterSpacing: 0 }}>
                {label}
              </Typography>
            ))}
          </Stack>
        </Stack>
      </Container>
    </Box>
  );
}

// -----------------------------------------------------------------------
// CAPABILITY TOUR
// -----------------------------------------------------------------------

function CapabilityTour() {
  return (
    <Section>
      <SectionHeader
        eyebrow={brand.capabilityEyebrow}
        title={brand.capabilityTitle}
        subtitle="Lovable, Base44, Bolt, Replit Agent, and v0 proved that prompting can create a working start. Ironflyer is built for the part after that: decisions, diffs, tests, security, spend, and deploy."
      />
      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)' },
        gap: { xs: 2, md: 2.5 },
      }}>
        {capabilityCards.map((card) => (
          <Box key={card.eyebrow} sx={{
            ...panelSx,
            p: { xs: 3, md: 4 },
            display: 'flex',
            flexDirection: 'column',
            gap: 2,
            minHeight: { xs: 260, md: 320 },
            position: 'relative',
            transition: `transform ${tokens.motion.base} ${tokens.motion.curve}, box-shadow ${tokens.motion.base} ${tokens.motion.curve}`,
            '&:hover': { transform: 'translateY(-2px)', boxShadow: '0 14px 36px rgba(13,14,15,0.10)' },
          }}>
            <Stack direction="row" justifyContent="space-between" alignItems="flex-start">
              <Typography variant="overline" sx={{ color: MUTED, fontWeight: 900, letterSpacing: '0.12em' }}>
                {card.eyebrow}
              </Typography>
              <Box sx={{
                width: 36, height: 36, borderRadius: '10px',
                bgcolor: '#0d0e0f', color: LIME,
                display: 'grid', placeItems: 'center',
              }}>
                {card.icon}
              </Box>
            </Stack>
            <Typography sx={{
              fontFamily: tokens.font.display,
              fontWeight: 400,
              fontSize: { xs: '1.95rem', md: '2.5rem' },
              lineHeight: 1, letterSpacing: 0,
              maxWidth: 520,
              color: INK,
            }}>
              {card.title}
            </Typography>
            <Typography sx={{ color: '#3a352d', fontWeight: 500, maxWidth: 540, lineHeight: 1.5, flex: 1 }}>
              {card.text}
            </Typography>
            <Chip
              label={card.chip}
              size="small"
              sx={{
                alignSelf: 'flex-start',
                bgcolor: INK,
                color: LIME,
                borderRadius: '999px',
                fontWeight: 900,
                px: 0.4,
                '& .MuiChip-label': { px: 1.4 },
              }}
            />
          </Box>
        ))}
      </Box>
    </Section>
  );
}

// -----------------------------------------------------------------------
// GATE EXAMPLES (real output)
// -----------------------------------------------------------------------

export function GateExamples() {
  return (
    <Section dark>
      <SectionHeader
        eyebrow={brand.gatesEyebrow}
        title={brand.gatesTitle}
        subtitle="A gate is not a badge. It is a blocking verdict with the evidence needed to repair the product."
        light
      />
      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)' },
        gap: { xs: 2, md: 2.5 },
      }}>
        {gateExamples.map((gate) => (
          <Box key={gate.name} sx={{
            borderRadius: 1,
            border: '1px solid rgba(244,240,232,0.1)',
            bgcolor: '#15161a',
            overflow: 'hidden',
            display: 'flex',
            flexDirection: 'column',
          }}>
            <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{
              px: 2.5, py: 1.6,
              borderBottom: '1px solid rgba(244,240,232,0.06)',
              bgcolor: '#0f1013',
            }}>
              <Stack direction="row" spacing={1.2} alignItems="center">
                <Box sx={{
                  width: 28, height: 28, borderRadius: 1,
                  bgcolor: 'rgba(229,255,0,0.16)',
                  color: LIME, display: 'grid', placeItems: 'center',
                }}>
                  {gate.icon}
                </Box>
                <Typography sx={{ color: '#f4f0e8', fontWeight: 900, fontSize: 15, letterSpacing: 0.2 }}>
                  {gate.title}
                </Typography>
              </Stack>
              <Stack direction="row" spacing={0.6} alignItems="center">
                {['#ff5f57', '#febc2e', '#28c840'].map((c) => (
                  <Box key={c} sx={{ width: 9, height: 9, borderRadius: 999, bgcolor: c, opacity: 0.62 }} />
                ))}
              </Stack>
            </Stack>
            <Box component="pre" sx={{
              m: 0, p: { xs: 2.2, md: 3 },
              fontFamily: tokens.font.mono,
              fontSize: { xs: 11.5, md: 12.5 },
              lineHeight: 1.55,
              color: '#d6cfbf',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              flex: 1,
              '& > *': { display: 'block' },
            }}>
              {gate.output}
            </Box>
            <Box sx={{
              px: { xs: 2.2, md: 3 }, py: 2,
              borderTop: '1px solid rgba(244,240,232,0.06)',
              bgcolor: 'rgba(229,255,0,0.04)',
            }}>
              <Typography variant="body2" sx={{ color: '#cfc7b8', fontWeight: 600, lineHeight: 1.5 }}>
                {gate.caption}
              </Typography>
            </Box>
          </Box>
        ))}
      </Box>
    </Section>
  );
}

// -----------------------------------------------------------------------
// CLOUD IDE PREVIEW
// -----------------------------------------------------------------------

function CloudIDEPreview() {
  return (
    <Section>
      <SectionHeader
        eyebrow="Workspace"
        title="A real runtime for real debugging."
        subtitle="Per-user Docker container, file API, full PTY terminal, live preview, and the same project state across browser and VSCode."
      />
      <Box sx={{
        ...panelSx,
        bgcolor: '#0d0e0f',
        color: ALABASTER,
        p: { xs: 1.5, md: 2 },
        boxShadow: '0 24px 56px rgba(13,14,15,0.26)',
      }}>
        <Stack direction="row" spacing={0.8} alignItems="center" sx={{ px: 1.4, py: 1.2 }}>
          {['#ff5f57', '#febc2e', '#28c840'].map((c) => (
            <Box key={c} sx={{ width: 11, height: 11, borderRadius: 999, bgcolor: c }} />
          ))}
          <Typography variant="caption" sx={{ ml: 1.4, color: '#9c968a', fontFamily: tokens.font.mono, fontSize: 12 }}>
            ironflyer ▸ workspace ▸ portal-mvp
          </Typography>
          <Box sx={{ flex: 1 }} />
          <Chip
            size="small"
            label="● live · 14% / cap"
            sx={{ bgcolor: 'rgba(229,255,0,0.12)', color: LIME, borderRadius: '999px', fontWeight: 800, fontSize: 11 }}
          />
        </Stack>
        <Box sx={{
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', md: '220px 1fr 320px' },
          gap: 1,
          minHeight: { xs: 380, md: 480 },
          fontFamily: tokens.font.mono,
          fontSize: 12.5,
        }}>
          <Box sx={{ bgcolor: '#16171b', borderRadius: 1, p: 2, color: '#bdb7ab' }}>
            <Typography variant="caption" sx={{ color: '#7d7770', textTransform: 'uppercase', letterSpacing: '0.14em' }}>Files</Typography>
            <Stack spacing={0.6} sx={{ mt: 1.4, fontFamily: tokens.font.mono, fontSize: 12.5 }}>
              {[
                '▸ apps/',
                '  ▸ web/',
                '    ▾ app/',
                '      • page.tsx',
                '      • layout.tsx',
                '      • portal/',
                '  ▾ api/',
                '    • auth.go',
                '    • portal.go',
                'docker-compose.yml',
                'README.md',
              ].map((f, i) => (
                <Box key={i} sx={{ pl: f.startsWith(' ') ? f.indexOf('•') > 0 ? 1.2 : 0.6 : 0, color: f.includes('portal') ? LIME : 'inherit' }}>
                  {f}
                </Box>
              ))}
            </Stack>
          </Box>
          <Box sx={{ bgcolor: '#0f1013', borderRadius: 1, p: 2.4, overflow: 'hidden' }}>
            <Typography variant="caption" sx={{ color: '#7d7770', textTransform: 'uppercase', letterSpacing: '0.14em' }}>portal/page.tsx</Typography>
            <Box component="pre" sx={{ m: 0, mt: 1.4, fontFamily: tokens.font.mono, color: '#d6cfbf', whiteSpace: 'pre' }}>
{`'use client';
import { useSession } from '@/lib/auth';
import { DocumentList } from './documents';

export default function PortalPage() {
  const { user } = useSession();
  if (!user) return null;
  return (
    `}<Box component="span" sx={{ color: LIME }}>{`<DocumentList ownerId={user.id} />`}</Box>{`
  );
}`}
            </Box>
          </Box>
          <Box sx={{ bgcolor: '#16171b', borderRadius: 1, p: 2.4, color: '#bdb7ab', display: 'flex', flexDirection: 'column', gap: 1.4 }}>
            <Typography variant="caption" sx={{ color: '#7d7770', textTransform: 'uppercase', letterSpacing: '0.14em' }}>Terminal</Typography>
            <Box component="pre" sx={{ m: 0, fontFamily: tokens.font.mono, color: '#d6cfbf', whiteSpace: 'pre-wrap', flex: 1 }}>
{`$ npm test
 PASS  apps/web/portal.spec.tsx
 PASS  apps/api/portal_test.go
 ─────────────────────────────
 Tests: 18 passed, 18 total
 Time:  3.42s

$ ironflyer gate run security
✓ secret_scan
✓ dep_audit
✓ owasp_a02
green — patch admitted`}
            </Box>
            <Chip
              size="small"
              icon={<Visibility sx={{ fontSize: 14 }} />}
              label="Live preview · :3000"
              sx={{ alignSelf: 'flex-start', bgcolor: 'rgba(120,219,255,0.16)', color: '#78dbff', borderRadius: '999px', fontWeight: 800 }}
            />
          </Box>
        </Box>
      </Box>
    </Section>
  );
}

// -----------------------------------------------------------------------
// BUDGET SPOTLIGHT
// -----------------------------------------------------------------------

function BudgetSpotlight() {
  return (
    <Section>
      <Box sx={{
        ...panelSx,
        bgcolor: '#0d0e0f',
        color: ALABASTER,
        p: { xs: 3, md: 6 },
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: '1.1fr 0.9fr' },
        gap: { xs: 4, md: 6 },
        alignItems: 'center',
      }}>
        <Box>
          <Typography variant="overline" sx={{ color: LIME, fontWeight: 900, letterSpacing: '0.14em' }}>
            The budget model
          </Typography>
          <Typography sx={{
            fontFamily: tokens.font.display,
            fontWeight: 400,
            fontSize: { xs: '2.6rem', md: '4.4rem' },
            lineHeight: 0.94,
            mt: 1.2,
            letterSpacing: 0,
          }}>
            The bill is part of the product.
          </Typography>
          <Typography sx={{ mt: 2.4, color: '#cfc7b8', maxWidth: 560, fontSize: { xs: 15, md: 17 }, lineHeight: 1.55, fontWeight: 500 }}>
            AI builders hide behind credits. Ironflyer shows the ledger: which gate spent money, which provider handled it, how close you are to the cap, and when the router changed models to protect the budget.
          </Typography>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ mt: 3.2 }}>
            <Button component={Link} href="/pricing" variant="contained" endIcon={<ArrowForward />} sx={{ bgcolor: LIME, color: INK, fontWeight: 800, borderRadius: '999px', px: 2.6 }}>
              See pricing
            </Button>
            <Button component={Link} href="/pricing#calculator" variant="outlined" endIcon={<TrendingUp />} sx={{ color: ALABASTER, borderColor: 'rgba(244,240,232,0.24)', borderRadius: '999px', px: 2.6 }}>
              Calculate your cost
            </Button>
          </Stack>
        </Box>
        <Box sx={{
          borderRadius: 1,
          bgcolor: 'rgba(244,240,232,0.04)',
          border: '1px solid rgba(244,240,232,0.1)',
          p: { xs: 2.5, md: 3.2 },
          fontFamily: tokens.font.mono,
        }}>
          <Typography variant="caption" sx={{ color: '#9c968a', textTransform: 'uppercase', letterSpacing: '0.14em' }}>
            Pro · this month
          </Typography>
          <Box sx={{ mt: 1.8 }}>
            {[
              { label: 'Subscription',        value: '$20.00', sub: 'paid'  },
              { label: 'Provider cost',       value: '$14.62', sub: '73% of cap' },
              { label: 'Margin',              value: '+$5.38', sub: 'company keeps', color: LIME },
            ].map((row) => (
              <Stack key={row.label} direction="row" justifyContent="space-between" alignItems="baseline" sx={{ py: 1.2, borderBottom: '1px dashed rgba(244,240,232,0.1)' }}>
                <Box>
                  <Typography sx={{ color: ALABASTER, fontWeight: 700, fontFamily: tokens.font.family }}>{row.label}</Typography>
                  <Typography variant="caption" sx={{ color: '#9c968a', fontFamily: tokens.font.family }}>{row.sub}</Typography>
                </Box>
                <Typography sx={{ color: row.color || ALABASTER, fontWeight: 800, fontSize: 22 }}>
                  {row.value}
                </Typography>
              </Stack>
            ))}
          </Box>
          <Box sx={{ mt: 2.4 }}>
            <Typography variant="caption" sx={{ color: '#9c968a', textTransform: 'uppercase', letterSpacing: '0.14em', display: 'block', mb: 1.2 }}>
              Top models
            </Typography>
            {[
              { name: 'claude-haiku-4.5',   pct: 55, cost: '$8.10' },
              { name: 'claude-sonnet-4.6',  pct: 35, cost: '$5.20' },
              { name: 'gpt-4o-mini',        pct: 10, cost: '$1.32' },
            ].map((m) => (
              <Box key={m.name} sx={{ mb: 1 }}>
                <Stack direction="row" justifyContent="space-between" sx={{ mb: 0.5 }}>
                  <Typography variant="caption" sx={{ color: ALABASTER, fontFamily: tokens.font.mono, fontSize: 12 }}>{m.name}</Typography>
                  <Typography variant="caption" sx={{ color: '#cfc7b8', fontFamily: tokens.font.mono, fontSize: 12 }}>{m.cost}</Typography>
                </Stack>
                <Box sx={{ height: 5, borderRadius: 999, bgcolor: 'rgba(244,240,232,0.06)', overflow: 'hidden' }}>
                  <Box sx={{ width: `${m.pct}%`, height: '100%', bgcolor: LIME }} />
                </Box>
              </Box>
            ))}
          </Box>
        </Box>
      </Box>
    </Section>
  );
}

// -----------------------------------------------------------------------
// TESTIMONIALS
// -----------------------------------------------------------------------

function TestimonialsSection() {
  return (
    <Section>
      <SectionHeader
        eyebrow={brand.socialEyebrow}
        title={brand.socialTitle}
        subtitle="The homepage should make a claim only when the product surface can show the evidence. These are the proof objects Ironflyer exposes instead of hiding the work behind a demo."
      />
      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: 'repeat(4, 1fr)' },
        gap: 2,
      }}>
        {proofPoints.map((point) => (
          <Box key={point.label} sx={{
            ...panelSx,
            bgcolor: '#fffaf1',
            border: '1px solid rgba(13,14,15,0.08)',
            p: { xs: 2.4, md: 3 },
            minHeight: 250,
            display: 'flex',
            flexDirection: 'column',
          }}>
            <Typography variant="overline" sx={{ color: MUTED, fontWeight: 900, letterSpacing: '0.12em', fontSize: 11.5 }}>
              {point.label}
            </Typography>
            <Typography sx={{
              fontFamily: tokens.font.display,
              fontSize: { xs: '2rem', md: '2.35rem' },
              lineHeight: 0.95,
              letterSpacing: 0,
              color: INK,
              mt: 1,
            }}>
              {point.value}
            </Typography>
            <Typography sx={{ mt: 'auto', pt: 3, color: MUTED, fontWeight: 600, lineHeight: 1.55 }}>
              {point.text}
            </Typography>
          </Box>
        ))}
      </Box>
    </Section>
  );
}

// -----------------------------------------------------------------------
// COMPARISON TABLE
// -----------------------------------------------------------------------

function ComparisonTable() {
  const competitors = ['Ironflyer', 'Base44', 'Lovable', 'v0/Bolt'] as const;
  return (
    <Section>
      <SectionHeader
        eyebrow={brand.compareEyebrow}
        title={brand.compareTitle}
        subtitle="Competitors made the first prompt feel powerful. Ironflyer is designed for the review that follows: can we ship this, own it, and explain it?"
      />
      <Box sx={{
        ...panelSx,
        overflow: 'auto',
        bgcolor: '#fffaf1',
        border: '1px solid rgba(13,14,15,0.08)',
      }}>
        <Box component="table" sx={{ width: '100%', minWidth: 760, borderCollapse: 'collapse', fontSize: { xs: 13.5, md: 15 } }}>
          <Box component="thead" sx={{ position: 'sticky', top: 0, zIndex: 1, bgcolor: '#fffaf1' }}>
            <Box component="tr">
              <Box component="th" sx={{
                textAlign: 'left', p: 2.2,
                borderBottom: '1px solid rgba(17,17,17,0.1)',
                color: MUTED, fontWeight: 800, letterSpacing: '0.1em',
                textTransform: 'uppercase', fontSize: 11.5,
              }}>
                Capability
              </Box>
              {competitors.map((name, i) => (
                <Box key={name} component="th" sx={{
                  textAlign: 'center', p: 2.2,
                  borderBottom: '1px solid rgba(17,17,17,0.1)',
                  fontWeight: 900, fontSize: 14,
                  color: i === 0 ? INK : '#3a352d',
                  bgcolor: i === 0 ? LIME : 'transparent',
                }}>
                  {name}
                </Box>
              ))}
            </Box>
          </Box>
          <Box component="tbody">
            {comparisonRows.map((row, idx) => (
              <Box component="tr" key={row.label} sx={{ bgcolor: idx % 2 === 0 ? 'transparent' : 'rgba(13,14,15,0.025)' }}>
                <Box component="td" sx={{
                  p: 2.2, borderBottom: '1px solid rgba(17,17,17,0.07)',
                  fontWeight: 700, color: INK,
                }}>
                  {row.label}
                </Box>
                {row.values.map((v, i) => (
                  <Box key={i} component="td" sx={{
                    p: 2.2, textAlign: 'center',
                    borderBottom: '1px solid rgba(17,17,17,0.07)',
                    fontWeight: i === 0 ? 800 : 600,
                    color: i === 0 ? INK : '#3a352d',
                    bgcolor: i === 0 ? 'rgba(229,255,0,0.18)' : 'transparent',
                    fontFamily: v === '—' ? tokens.font.mono : tokens.font.family,
                  }}>
                    {v}
                  </Box>
                ))}
              </Box>
            ))}
          </Box>
        </Box>
      </Box>
      <Typography variant="caption" sx={{
        display: 'block', mt: 1.8, color: MUTED,
        textAlign: 'center', maxWidth: 720, mx: 'auto',
      }}>
        Snapshot updated {new Date().toISOString().slice(0, 7)}. When a competitor ships, we update the row honestly — open an issue if something shifted.
      </Typography>
    </Section>
  );
}

// -----------------------------------------------------------------------
// USE CASE GRID
// -----------------------------------------------------------------------

function UseCaseGrid() {
  const row = (title: string, items: { tag: string; label: string }[]) => (
    <Box>
      <Typography variant="overline" sx={{ color: MUTED, fontWeight: 900, letterSpacing: '0.14em' }}>{title}</Typography>
      <Stack direction="row" spacing={1} flexWrap="wrap" sx={{ mt: 1.4, rowGap: 1 }}>
        {items.map((it) => (
          <Chip
            key={it.tag}
            label={it.label}
            component={Link}
            href={`/solutions?filter=${encodeURIComponent(it.tag)}`}
            clickable
            sx={{
              bgcolor: '#fffaf1', color: INK,
              border: '1px solid rgba(13,14,15,0.1)',
              fontWeight: 800, px: 0.3,
              borderRadius: '999px',
              '& .MuiChip-label': { px: 1.6 },
              '&:hover': { bgcolor: LIME, color: INK, borderColor: 'transparent' },
              transition: `background-color ${tokens.motion.base} ${tokens.motion.curve}`,
            }}
          />
        ))}
      </Stack>
    </Box>
  );
  return (
    <Section>
      <SectionHeader eyebrow="Use cases" title="Pick the row that looks like you." />
      <Stack spacing={{ xs: 3, md: 4 }}>
        {row('By industry', useCasesByIndustry)}
        {row('By role', useCasesByRole)}
      </Stack>
    </Section>
  );
}

// -----------------------------------------------------------------------
// FAQ
// -----------------------------------------------------------------------

const faqs = [
  {
    q: 'How is Ironflyer different from Base44, Lovable, Bolt, Replit Agent, or v0?',
    a: 'Those products are strong at the first prompt: describe an idea and get something running. Ironflyer is built for the next review: is the spec complete, does the code build, did security pass, what did it cost, and can we deploy it with ownership?',
  },
  {
    q: 'How does the budget work?',
    a: 'Each plan has a flat subscription and a cost cap — that’s what Ironflyer is willing to spend on provider calls. A live budget card shows your $ burn vs cap and the top three models. No credit packs to top up. When the cap is reached, the router downgrades or pauses cleanly.',
  },
  {
    q: 'Do I need to know how to code?',
    a: 'No. You can start from plain English and let the agents plan, build, test, and deploy. Technical teams get the deeper control: a real repo, a Linux workspace, terminal access, and reviewable patches for every change.',
  },
  {
    q: 'Can I bring it to my own cloud?',
    a: 'Yes. The whole stack ships as a Helm chart you install in any Kubernetes cluster — orchestrator, runtime sandboxes, web, Postgres, optional ingress + TLS. DEPLOY.md walks you through it.',
  },
  {
    q: 'How do you handle security?',
    a: 'The Security gate scans every patch for credentials, dependency risk, and OWASP-shaped issues. Workspaces run in per-user Docker containers, secrets live in controlled secret stores, and the AI never writes files directly.',
  },
  {
    q: 'Who owns the code?',
    a: 'You do. Connect GitHub and the loop pushes there; export from the workspace and you walk away with a full repo plus the Dockerfile and Helm chart. No platform lock-in on the artifact.',
  },
];

function FAQSection() {
  return (
    <Section>
      <SectionHeader eyebrow="FAQ" title="Questions builders ask before the first prompt." />
      <Box sx={{ maxWidth: 940, mx: 'auto' }}>
        {faqs.map((item) => (
          <Box
            key={item.q}
            component="details"
            sx={{
              borderBottom: '1px solid rgba(13,14,15,0.1)',
              py: 2.4,
              cursor: 'pointer',
              '& > summary': { listStyle: 'none', outline: 'none' },
              '& > summary::-webkit-details-marker': { display: 'none' },
              '&[open] .faq-marker': { transform: 'rotate(45deg)' },
            }}
          >
            <Box component="summary" sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 2,
            }}>
              <Typography sx={{
                fontSize: { xs: '1.1rem', md: '1.32rem' },
                fontWeight: 700,
                letterSpacing: 0,
                color: INK,
              }}>
                {item.q}
              </Typography>
              <Box className="faq-marker" sx={{
                width: 22, height: 22, flexShrink: 0,
                position: 'relative',
                transition: `transform ${tokens.motion.base} ${tokens.motion.curve}`,
                '&::before, &::after': {
                  content: '""', position: 'absolute',
                  background: INK, left: '50%', top: '50%',
                  transform: 'translate(-50%, -50%)',
                },
                '&::before': { width: 14, height: 2 },
                '&::after':  { width: 2,  height: 14 },
              }} />
            </Box>
            <Typography sx={{
              mt: 1.6, color: '#3a352d',
              fontSize: { xs: '0.96rem', md: '1.02rem' },
              lineHeight: 1.62,
              maxWidth: 800,
            }}>
              {item.a}
            </Typography>
          </Box>
        ))}
      </Box>
    </Section>
  );
}

// -----------------------------------------------------------------------
// FINAL CTA
// -----------------------------------------------------------------------

export function FinalCta({
  primary = brand.heroCtaPrimary,
  secondary = brand.heroCtaSecondary,
  primaryHref = '/app',
  secondaryHref = '/product',
}: {
  primary?: string;
  secondary?: string;
  primaryHref?: string;
  secondaryHref?: string;
}) {
  return (
    <Section>
      <Box sx={{
        ...panelSx,
        bgcolor: INK, color: ALABASTER,
        p: { xs: 4, md: 7 },
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', md: '1.2fr 0.8fr' },
        gap: 3,
        overflow: 'hidden',
      }}>
        <Box>
          <Typography sx={{
            fontFamily: tokens.font.display, fontWeight: 400,
            letterSpacing: 0,
            fontSize: { xs: '2.6rem', md: '4.2rem' }, lineHeight: 0.95,
          }}>
            {brand.ctaTitle}
          </Typography>
          <Typography sx={{ mt: 2.2, color: '#cfc7b8', fontWeight: 500, maxWidth: 560, lineHeight: 1.55, fontSize: { xs: 15, md: 17 } }}>
            {brand.ctaSubtitle}
          </Typography>
        </Box>
        <Stack direction={{ xs: 'column', sm: 'row', md: 'column' }} spacing={1.5} justifyContent="center">
          <Button component={Link} href={primaryHref} variant="contained" size="large" endIcon={<RocketLaunch />} sx={{ bgcolor: LIME, color: INK, py: 1.6, borderRadius: '999px', fontWeight: 800, fontSize: 16 }}>
            {primary}
          </Button>
          <Button component={Link} href={secondaryHref} variant="outlined" size="large" endIcon={<ArrowForward />} sx={{ color: ALABASTER, borderColor: 'rgba(244,240,232,0.28)', py: 1.6, borderRadius: '999px', fontWeight: 800, fontSize: 16 }}>
            {secondary}
          </Button>
        </Stack>
      </Box>
    </Section>
  );
}

// -----------------------------------------------------------------------
// SHARED PRIMITIVES
// -----------------------------------------------------------------------

export function Section({ children, id, dark }: { children: React.ReactNode; id?: string; dark?: boolean }) {
  return (
    <Box
      id={id}
      component="section"
      sx={{
        py: { xs: 7, md: 11 },
        bgcolor: dark ? INK : ALABASTER,
        color: dark ? ALABASTER : INK,
        scrollMarginTop: 80,
      }}
    >
      <Container maxWidth="xl">{children}</Container>
    </Box>
  );
}

export function SectionHeader({
  eyebrow, title, subtitle, action, compact = false, light = false,
}: {
  eyebrow: string;
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
  compact?: boolean;
  light?: boolean;
}) {
  return (
    <Stack
      direction={{ xs: 'column', md: action ? 'row' : 'column' }}
      justifyContent="space-between"
      alignItems={{ xs: 'flex-start', md: action ? 'flex-end' : 'flex-start' }}
      spacing={2}
      sx={{ mb: compact ? 3 : { xs: 4, md: 6 } }}
    >
      <Box sx={{ maxWidth: 980 }}>
        <Typography variant="overline" sx={{ color: light ? LIME : '#6c7a00', fontWeight: 900, letterSpacing: '0.14em', fontSize: 12.5 }}>
          {eyebrow}
        </Typography>
        <Typography sx={{
          mt: 1,
          fontFamily: tokens.font.display,
          fontWeight: 400,
          fontSize: compact ? { xs: '2.3rem', md: '3.4rem' } : { xs: '2.6rem', md: '4.6rem' },
          lineHeight: 0.92,
          letterSpacing: 0,
          color: light ? ALABASTER : INK,
          textWrap: 'balance',
        }}>
          {title}
        </Typography>
        {subtitle && (
          <Typography sx={{
            mt: 2,
            color: light ? '#cfc7b8' : '#3a352d',
            fontSize: { xs: 15, md: 17 },
            fontWeight: 500,
            maxWidth: 720,
            lineHeight: 1.5,
          }}>
            {subtitle}
          </Typography>
        )}
      </Box>
      {action}
    </Stack>
  );
}

export function PageHero({
  eyebrow,
  title,
  subtitle,
  primary,
  primaryHref,
  secondary,
  secondaryHref,
}: {
  eyebrow: string;
  title: string;
  subtitle: string;
  primary?: string;
  primaryHref?: string;
  secondary?: string;
  secondaryHref?: string;
}) {
  return (
    <Box sx={{ bgcolor: ALABASTER, pt: { xs: 5, md: 8 }, pb: { xs: 5, md: 7 } }}>
      <Container maxWidth="xl">
        <Stack spacing={3} alignItems="flex-start" sx={{ maxWidth: 980 }}>
          <Typography variant="overline" sx={{ color: '#6c7a00', fontWeight: 900, letterSpacing: '0.14em', fontSize: 13 }}>
            {eyebrow}
          </Typography>
          <Typography component="h1" sx={{
            fontFamily: tokens.font.display, fontWeight: 400,
            fontSize: { xs: '3rem', md: '6rem' },
            lineHeight: 0.9, letterSpacing: 0,
            color: INK,
            textWrap: 'balance',
          }}>
            {title}
          </Typography>
          <Typography sx={{ color: '#3a352d', fontSize: { xs: '1.1rem', md: '1.32rem' }, fontWeight: 500, maxWidth: 780, lineHeight: 1.5 }}>
            {subtitle}
          </Typography>
          {(primary || secondary) && (
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ pt: 1 }}>
              {primary && primaryHref && (
                <Button component={Link} href={primaryHref} variant="contained" endIcon={<ArrowForward />} sx={{ bgcolor: LIME, color: INK, fontWeight: 800, px: 2.6, py: 1.4, borderRadius: '999px' }}>
                  {primary}
                </Button>
              )}
              {secondary && secondaryHref && (
                <Button component={Link} href={secondaryHref} variant="outlined" endIcon={<ArrowForward />} sx={{ color: INK, borderColor: 'rgba(13,14,15,0.2)', fontWeight: 800, px: 2.6, py: 1.4, borderRadius: '999px' }}>
                  {secondary}
                </Button>
              )}
            </Stack>
          )}
        </Stack>
      </Container>
    </Box>
  );
}

// -----------------------------------------------------------------------
// PRODUCT PAGE
// -----------------------------------------------------------------------

const finisherLoopSteps = [
  { name: 'Spec',          text: 'Turn the prompt into user stories, a data model, and acceptance criteria. Stops on ambiguity.', icon: <WorkOutline /> },
  { name: 'UX',            text: 'Generate a navigation tree, screens with empty/loading/error states, and a design lint pass.', icon: <Build /> },
  { name: 'Architecture',  text: 'Pick the stack, draft the backend contracts, set the deploy target. Reviewed before code.',     icon: <Settings /> },
  { name: 'Code',          text: 'Patches land via the patch engine. No direct file writes. Build + vet + tsc must be green.',    icon: <Code /> },
  { name: 'Tests',         text: 'Run the project test suite. Failures block; the agent re-plans and patches until green.',       icon: <CheckCircle /> },
  { name: 'Security',      text: 'Secret scan, dep audit, OWASP checks on the diff. Findings come with suggested fixes.',         icon: <Shield /> },
  { name: 'Budget',        text: 'Every model call charges the ledger. Cap nears → router downgrades. No surprise bills.',        icon: <Insights /> },
  { name: 'Deploy',        text: 'Helm chart, healthchecks, secrets, observability — refuses to ship until everything passes.',    icon: <RocketLaunch /> },
];

export function ProductPage() {
  return (
    <MarketingShell>
      <PageHero
        eyebrow="Product"
        title="The Finisher Loop, in detail."
        subtitle="Nine gates. One workspace. The AI never writes files directly — every change is a patch the loop has to approve."
        primary="Start a project"
        primaryHref="/app"
        secondary="See pricing"
        secondaryHref="/pricing"
      />
      <Section>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)' }, gap: { xs: 2, md: 2 } }}>
          {finisherLoopSteps.map((step, idx) => (
            <Box key={step.name} sx={{
              ...panelSx,
              p: { xs: 3, md: 4 },
              display: 'grid',
              gridTemplateColumns: '52px 1fr',
              gap: 2.4,
              alignItems: 'flex-start',
            }}>
              <Box sx={{
                width: 52, height: 52, borderRadius: '12px',
                bgcolor: INK, color: LIME,
                display: 'grid', placeItems: 'center', flexShrink: 0,
              }}>
                {step.icon}
              </Box>
              <Box>
                <Typography variant="overline" sx={{ color: MUTED, fontWeight: 900, letterSpacing: '0.14em', fontSize: 12 }}>
                  Gate {String(idx + 1).padStart(2, '0')}
                </Typography>
                <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: { xs: '1.7rem', md: '2.1rem' }, lineHeight: 1, mt: 0.4 }}>
                  {step.name}
                </Typography>
                <Typography sx={{ mt: 1.4, color: '#3a352d', fontWeight: 500, lineHeight: 1.55 }}>
                  {step.text}
                </Typography>
              </Box>
            </Box>
          ))}
        </Box>
      </Section>
      <GateExamples />
      <CloudIDEPreview />
      <Section>
        <SectionHeader
          eyebrow="VSCode extension"
          title="Use the loop from your editor."
          subtitle="The Ironflyer VSCode extension is a thin client over the orchestrator. Sign in once with SecretStorage, get the chat panel, gate stream, and patch lifecycle natively inside the IDE you already use."
          action={<Chip label="Available now in private beta" sx={{ bgcolor: LIME, color: INK, fontWeight: 800, borderRadius: '999px' }} />}
        />
        <Box sx={{
          ...panelSx,
          p: { xs: 2.5, md: 4 },
          display: 'grid',
          gridTemplateColumns: { xs: '1fr', md: '1fr 1fr 1fr' },
          gap: 2,
        }}>
          {[
            { title: 'Same SSE stream', text: 'The extension subscribes to the same gate stream the web client uses. No duplication, no protocol drift.' },
            { title: 'SecretStorage auth', text: 'Token sits in VSCode SecretStorage, refreshed via the URI handler. No copy-paste tokens, no .env leak.' },
            { title: 'Patch lifecycle UI', text: 'Every patch shows its source gate, diff preview, and admit/reject affordance — like an integrated code review.' },
          ].map((c) => (
            <Box key={c.title} sx={{ bgcolor: '#fffaf1', borderRadius: 1, p: 3 }}>
              <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: 22, lineHeight: 1, mb: 1.4 }}>{c.title}</Typography>
              <Typography variant="body2" sx={{ color: MUTED, fontWeight: 600, lineHeight: 1.55 }}>{c.text}</Typography>
            </Box>
          ))}
        </Box>
      </Section>
      <Section id="changelog">
        <SectionHeader
          eyebrow="Changelog"
          title="What shipped recently."
        />
        <Stack spacing={2}>
          {[
            { date: '2026-05', title: 'Effort dial (Lite / Economy / Power) in chat', text: 'Override the router’s cost-quality choice per turn. The dial is sticky per project and surfaces what changes.' },
            { date: '2026-04', title: 'VSCode extension — private beta', text: 'Native chat + gate stream + patch lifecycle inside VSCode. URI-handler auth, no token copy-paste.' },
            { date: '2026-03', title: 'Live budget card with top-3 models',          text: 'Shows live $ burn vs cap, the three models eating most of it, and which gate spent the most.' },
            { date: '2026-02', title: 'Per-user Docker runtime + PTY WebSocket',     text: 'Replaces the mock driver in production. Real Linux sandbox, terminal, and file API per project.' },
          ].map((entry) => (
            <Box key={entry.title} sx={{ ...panelSx, p: { xs: 2.4, md: 3 }, display: 'grid', gridTemplateColumns: { xs: '1fr', md: '120px 1fr' }, gap: 2 }}>
              <Typography variant="caption" sx={{ color: MUTED, fontWeight: 800, fontFamily: tokens.font.mono, fontSize: 12.5 }}>{entry.date}</Typography>
              <Box>
                <Typography sx={{ fontWeight: 800, fontSize: 17, color: INK }}>{entry.title}</Typography>
                <Typography variant="body2" sx={{ mt: 0.6, color: MUTED, fontWeight: 500, lineHeight: 1.55 }}>{entry.text}</Typography>
              </Box>
            </Box>
          ))}
        </Stack>
      </Section>
      <FinalCta />
    </MarketingShell>
  );
}

// -----------------------------------------------------------------------
// SOLUTIONS PAGE
// -----------------------------------------------------------------------

const solutionStories = [
  {
    id: 'mvp',
    eyebrow: 'For founders',
    title: 'Ship the MVP your investors are actually expecting.',
    subtitle: 'You pitched a product. Don’t deliver a prototype. Ironflyer takes the spec to a working SaaS in a week — auth, billing, dashboard, deploy — and the gates make sure it survives a code review.',
    points: [
      { label: 'Day 1', text: 'Auth, Postgres, billing wired. Custom domain configured. Deploy gate green.' },
      { label: 'Day 3', text: 'Two end-to-end flows shipping behind feature flags. Test suite covers the happy paths.' },
      { label: 'Day 7', text: 'Pricing page, waitlist, analytics events, observability dashboard, security gate clean.' },
    ],
    quote: { text: 'We had a demo on day 4 and a stripe-charging product on day 9. The gates caught a permissions bug I would have shipped.', author: 'Lead engineer, pre-seed SaaS' },
  },
  {
    id: 'internal',
    eyebrow: 'For ops & internal teams',
    title: 'Internal tools your team won’t silently hate.',
    subtitle: 'Approval workflows, dense ops dashboards, CSV importers, role-aware admin consoles. The kind of internal apps every company needs and no one wants to build twice.',
    points: [
      { label: 'Roles', text: 'JWT auth + role-based access baked in. Audit log on every state change.' },
      { label: 'Data',  text: 'Postgres schemas generated from the spec. CSV import/export wired with validation.' },
      { label: 'UI',    text: 'Dense table views, inline edit, bulk actions — not toy no-code cards.' },
    ],
    quote: { text: 'We replaced two no-code tools and a half-built Retool with one Ironflyer workspace. Engineering wasn’t blocked, ops shipped what they needed.', author: 'VP Operations, healthtech' },
  },
  {
    id: 'agency',
    eyebrow: 'For agencies & client work',
    title: 'Repeatable client projects that close on a Friday.',
    subtitle: 'Spin up a project per client. Hand over a real repo at the end. The Helm chart means it deploys to their cloud, not yours. Margins improve when the gates stop catching issues in week six.',
    points: [
      { label: 'Per-client workspace', text: 'Isolated sandbox, isolated repo, isolated budget. Nothing leaks between accounts.' },
      { label: 'White-label deploy',   text: 'Strip the Ironflyer badge on Pro and ship to the client’s domain or their AWS account.' },
      { label: 'Handover-ready',       text: 'The project leaves as a Git repo + Dockerfile + Helm chart — no lock-in argument with the client.' },
    ],
    quote: { text: 'We doubled our client throughput in a quarter. The gates became our QA team.', author: 'Founder, EU product studio' },
  },
  {
    id: 'ai',
    eyebrow: 'For internal AI / platform teams',
    title: 'A self-hosted AI dev platform for your whole org.',
    subtitle: 'Bring Ironflyer to your own Kubernetes cluster via Helm. Hook it to your provider keys, your secrets, your data residency. Give every team a sandboxed workspace with budgets you control.',
    points: [
      { label: 'Helm chart',       text: 'Orchestrator, runtime, web, Postgres, ingress + TLS — all in one chart. DEPLOY.md is the runbook.' },
      { label: 'Provider routing', text: 'Anthropic, OpenAI, on-device ONNX. Configure the routing policy by team or by capability.' },
      { label: 'Budget per team',  text: 'Cost caps and ledgers per workspace, so finance can see what each team spent and where.' },
    ],
    quote: { text: 'Procurement-friendly because we own the data plane. Developer-friendly because the chat-to-patch UX is as good as anything in the cloud.', author: 'Platform lead, Series C SaaS' },
  },
];

export function SolutionsPage() {
  return (
    <MarketingShell>
      <PageHero
        eyebrow="Solutions"
        title="Four shapes of work Ironflyer ships best."
        subtitle="Concrete narratives, not industry buzzwords. Pick the closest one and read the timeline."
        primary="Start your project"
        primaryHref="/app"
      />
      {solutionStories.map((story, idx) => (
        <Section key={story.id} id={story.id} dark={idx % 2 === 1}>
          <Box sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
            gap: { xs: 4, md: 6 },
            alignItems: 'start',
          }}>
            <Box>
              <Typography variant="overline" sx={{ color: idx % 2 === 1 ? LIME : '#6c7a00', fontWeight: 900, letterSpacing: '0.14em' }}>
                {story.eyebrow}
              </Typography>
              <Typography sx={{
                fontFamily: tokens.font.display, fontWeight: 400,
                fontSize: { xs: '2.4rem', md: '3.6rem' },
                lineHeight: 0.94, letterSpacing: 0,
                mt: 1.2,
                color: idx % 2 === 1 ? ALABASTER : INK,
              }}>
                {story.title}
              </Typography>
              <Typography sx={{ mt: 2, color: idx % 2 === 1 ? '#cfc7b8' : '#3a352d', fontSize: { xs: 15, md: 17 }, fontWeight: 500, lineHeight: 1.55 }}>
                {story.subtitle}
              </Typography>
              <Box sx={{
                mt: 4, p: 3, borderRadius: 1,
                bgcolor: idx % 2 === 1 ? 'rgba(244,240,232,0.04)' : 'rgba(13,14,15,0.04)',
                border: `1px solid ${idx % 2 === 1 ? 'rgba(244,240,232,0.1)' : 'rgba(13,14,15,0.08)'}`,
              }}>
                <FormatQuote sx={{ color: LIME, fontSize: 22 }} />
                <Typography sx={{ mt: 1, fontSize: 17, fontWeight: 600, lineHeight: 1.5, color: idx % 2 === 1 ? ALABASTER : INK }}>
                  “{story.quote.text}”
                </Typography>
                <Typography variant="caption" sx={{ mt: 1.5, display: 'block', color: idx % 2 === 1 ? '#9c968a' : MUTED, fontWeight: 700 }}>
                  — {story.quote.author}
                </Typography>
              </Box>
            </Box>
            <Stack spacing={2}>
              {story.points.map((p) => (
                <Box key={p.label} sx={{
                  borderRadius: 1,
                  bgcolor: idx % 2 === 1 ? 'rgba(244,240,232,0.04)' : '#ece5d4',
                  p: 3,
                  border: `1px solid ${idx % 2 === 1 ? 'rgba(244,240,232,0.08)' : 'transparent'}`,
                }}>
                  <Stack direction="row" spacing={2} alignItems="flex-start">
                    <Box sx={{
                      width: 44, height: 44, borderRadius: '10px',
                      bgcolor: LIME, color: INK,
                      display: 'grid', placeItems: 'center',
                      fontWeight: 900,
                      fontFamily: tokens.font.display, fontSize: 18,
                    }}>
                      <CheckCircle sx={{ fontSize: 22 }} />
                    </Box>
                    <Box sx={{ flex: 1 }}>
                      <Typography sx={{ fontWeight: 800, fontSize: 16.5, color: idx % 2 === 1 ? ALABASTER : INK }}>
                        {p.label}
                      </Typography>
                      <Typography variant="body2" sx={{ mt: 0.6, color: idx % 2 === 1 ? '#cfc7b8' : MUTED, fontWeight: 500, lineHeight: 1.55 }}>
                        {p.text}
                      </Typography>
                    </Box>
                  </Stack>
                </Box>
              ))}
            </Stack>
          </Box>
        </Section>
      ))}
      <FinalCta />
    </MarketingShell>
  );
}

// -----------------------------------------------------------------------
// TEMPLATES PAGE
// -----------------------------------------------------------------------

const templateGallery = [
  { title: 'SaaS Dashboard',     desc: 'Multi-tenant SaaS with auth, teams, Stripe billing, usage analytics, admin settings.', tag: 'SaaS',          prompt: 'Build a production-ready SaaS app with auth, teams, Stripe billing, usage analytics, admin settings, onboarding flow, and a one-click deploy gate.', accent: LIME },
  { title: 'AI Chatbot',         desc: 'RAG-powered support assistant with doc upload, session memory, human hand-off.',         tag: 'AI',            prompt: 'Build an AI customer-support chatbot with retrieval over uploaded docs, session memory, hand-off to human, and an admin analytics dashboard.', accent: '#78dbff' },
  { title: 'Two-sided Marketplace', desc: 'Listings, search, messaging, Stripe Connect escrow payouts, trust scoring.',        tag: 'Marketplace',   prompt: 'Build a two-sided marketplace with listings, search filters, messaging, Stripe Connect escrow payouts, and a trust-score profile page.', accent: '#ff6c3a' },
  { title: 'Internal Admin',     desc: 'Role-based approvals, audit log, dense tables, CSV in/out, Postgres schema.',           tag: 'Internal',      prompt: 'Build an internal operations tool with role-based approvals, audit history, dense table UI, CSV export, and Postgres for the data model.', accent: '#8b5cff' },
  { title: 'Client Portal',      desc: 'Document uploads, threaded messaging, project status, role-aware access.',              tag: 'Portal',        prompt: 'Build a client portal with auth, document uploads, project status, threaded messaging, role-aware access, and an admin console.', accent: '#79e07a' },
  { title: 'Launch Site',        desc: 'Hero, waitlist, pricing tiers, FAQ, social proof, analytics events.',                   tag: 'Marketing',     prompt: 'Build a product launch site with hero, waitlist form, pricing, FAQ, social proof, and analytics events wired to the dataLayer.', accent: '#ffc400' },
  { title: 'E-commerce',         desc: 'Storefront, cart, Stripe checkout, order admin, inventory dashboard.',                  tag: 'E-commerce',    prompt: 'Build an e-commerce site with product catalog, cart, Stripe checkout, order admin, inventory dashboard, and customer accounts.', accent: '#671dfc' },
  { title: 'Booking System',     desc: 'Calendar, recurring slots, payment hold, notifications, no-show tracking.',             tag: 'Operations',    prompt: 'Build a booking system with calendar views, recurring slots, payment hold via Stripe, email + SMS notifications, and no-show tracking.', accent: '#ff1818' },
  { title: 'Analytics Dashboard', desc: 'Event ingest, role-aware metrics, segment filters, scheduled reports.',                tag: 'Analytics',     prompt: 'Build an analytics dashboard with event ingest, role-aware metric views, segment filters, scheduled reports, and a public-share link mode.', accent: LIME },
  { title: 'AI Forge',           desc: 'Prompt + model registry, agent runs, latency + cost panels across providers.',          tag: 'AI ops',        prompt: 'Build an internal AI ops console with a prompt registry, agent run history, latency + cost charts per provider, and an evals leaderboard.', accent: '#78dbff' },
  { title: 'Knowledge Base',     desc: 'Editor, search, role-aware drafts, public + private spaces, comments.',                 tag: 'Editorial',     prompt: 'Build a knowledge base with a block editor, full-text search, role-aware drafts, public + private spaces, and threaded comments.', accent: '#ff6c3a' },
  { title: 'Subscription Manager', desc: 'Customer portal, plan changes, invoice history, dunning, churn signals.',            tag: 'Billing',       prompt: 'Build a subscription manager with a customer-facing portal, plan changes, invoice history, dunning workflows, and a churn-signals dashboard.', accent: '#8b5cff' },
];

export function TemplatesPage() {
  return (
    <MarketingShell>
      <PageHero
        eyebrow="Templates"
        title="Twelve starting points. Pick one, the prompt fills itself."
        subtitle="Each template is a thoroughly-written prompt that runs through the same finisher gates as a from-scratch project. Click a card; the prompt box opens pre-filled."
      />
      <Section>
        <TemplatesGrid items={templateGallery} />
      </Section>
      <FinalCta />
    </MarketingShell>
  );
}

// -----------------------------------------------------------------------
// PRICING PAGE
// -----------------------------------------------------------------------

export function PricingPage() {
  return (
    <MarketingShell>
      <PageHero
        eyebrow="Pricing"
        title="One subscription. No credit packs. Live margin."
        subtitle="Most builders sell credit packs that vanish at 2 AM. Ironflyer charges a flat subscription, absorbs provider cost up to a published cap, and shows you the live ledger. The company makes a real margin; you never get a surprise bill."
      />
      <Suspense fallback={null}>
        <Box sx={{ bgcolor: ALABASTER }}>
          <Container maxWidth="xl" sx={{ pt: { xs: 0, md: 0 }, pb: 0 }}>
            <BillingStatusBanner compact />
          </Container>
        </Box>
      </Suspense>
      <PricingTiersSection />
      <Section id="calculator">
        <SectionHeader
          eyebrow="Cost calculator"
          title="Estimate your monthly spend before you commit."
          subtitle="Rough math with current Anthropic + OpenAI list prices. Real spend in your workspace is always lower thanks to capability-tagged routing."
        />
        <PricingCalculator />
      </Section>
      <PricingFAQ />
      <FinalCta primary="Start free" primaryHref="/app" secondary="Talk to sales" secondaryHref="/enterprise" />
    </MarketingShell>
  );
}

const pricingPlans = [
  {
    name: 'Starter',
    tier: 'free' as const,
    monthly: 0,
    yearly: 0,
    badge: 'Free forever',
    text: 'For exploring the loop and validating an idea has a real product shape.',
    cta: 'Start free',
    features: [
      'Starter build credits ($3 cost cap / mo)',
      'Public templates + projects',
      'Cloud IDE on shared runtime',
      'Ironflyer badge stays on',
      'GitHub push enabled',
    ],
  },
  {
    name: 'Pro',
    tier: 'pro' as const,
    monthly: 20,
    yearly: 16,
    badge: 'Most popular',
    text: 'For founders and solo builders shipping real MVPs with budget control.',
    cta: 'Go Pro',
    features: [
      '$15 cost cap / mo of provider spend',
      'Private projects + custom domains',
      'Per-user Docker sandbox',
      'Remove Ironflyer badge',
      'Multi-provider router (Anthropic + OpenAI)',
      'VSCode extension access',
    ],
  },
  {
    name: 'Team',
    tier: 'team' as const,
    monthly: 40,
    yearly: 32,
    badge: 'Scale together',
    text: 'For teams and agencies sharing workspaces, templates, and approval gates.',
    cta: 'Create team',
    features: [
      'Pooled cost cap (per-seat ledger)',
      '5 seats included, $8 / mo per extra',
      'Roles + approval gates',
      'Reusable team templates',
      'Audit log + SAML SSO (paid add-on)',
      'Priority email + Slack support',
    ],
    highlight: true,
  },
  {
    name: 'Enterprise',
    tier: 'enterprise' as const,
    monthly: null,
    yearly: null,
    badge: 'Procurement-ready',
    text: 'For organizations that need SSO, audit logs, private deploy, and an SLA.',
    cta: 'Contact sales',
    features: [
      'SSO (SAML / OIDC) + SCIM provisioning',
      'Self-hosted Helm chart + on-prem option',
      'Custom connectors + private model routing',
      'Dedicated onboarding + named CSM',
      'SOC 2 in-progress; DPA + custom MSA',
      '99.9% SLA',
    ],
  },
];

function PricingTiersSection() {
  return (
    <Section>
      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(4, 1fr)' },
        gap: 1.5,
      }}>
        {pricingPlans.map((plan) => {
          const dark = plan.highlight;
          return (
            <Box key={plan.name} sx={{
              ...panelSx,
              p: 3.2, minHeight: 520, display: 'flex', flexDirection: 'column',
              bgcolor: dark ? INK : '#ece5d4',
              color: dark ? ALABASTER : INK,
              position: 'relative',
              overflow: 'hidden',
              border: dark ? `2px solid ${LIME}` : '2px solid transparent',
            }}>
              <Chip
                label={plan.badge}
                sx={{
                  width: 'fit-content',
                  borderRadius: '999px',
                  bgcolor: dark ? 'rgba(229,255,0,0.18)' : '#d9cfbd',
                  color: dark ? LIME : INK,
                  fontWeight: 900,
                  position: 'relative',
                }}
              />
              <Typography variant="overline" sx={{ mt: 2.2, color: dark ? LIME : MUTED, fontWeight: 900, letterSpacing: '0.14em' }}>
                {plan.name}
              </Typography>
              <Stack direction="row" alignItems="baseline" spacing={0.6} sx={{ mt: 1, position: 'relative' }}>
                <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: { xs: '3rem', md: '3.6rem' }, letterSpacing: 0, lineHeight: 1 }}>
                  {plan.monthly === null ? 'Custom' : `$${plan.monthly}`}
                </Typography>
                {plan.monthly !== null && (
                  <Typography variant="caption" sx={{ color: dark ? '#cfc7b8' : MUTED, fontWeight: 800 }}>/ month</Typography>
                )}
              </Stack>
              <Typography variant="body2" sx={{ mt: 1.8, color: dark ? '#cfc7b8' : MUTED, fontWeight: 600, lineHeight: 1.55, position: 'relative' }}>
                {plan.text}
              </Typography>
              <Stack spacing={1.2} sx={{ mt: 2.5, mb: 3, flex: 1, position: 'relative' }}>
                {plan.features.map((feature) => (
                  <Stack key={feature} direction="row" spacing={1} alignItems="flex-start">
                    <CheckCircle sx={{ fontSize: 17, color: LIME, mt: 0.2, flexShrink: 0 }} />
                    <Typography variant="caption" sx={{ color: dark ? '#eee7db' : INK, fontWeight: 700, fontSize: 13, lineHeight: 1.45 }}>
                      {feature}
                    </Typography>
                  </Stack>
                ))}
              </Stack>
              <Box sx={{ position: 'relative' }}>
                {plan.tier === 'free' ? (
                  <Button component={Link} href="/app" variant="contained" fullWidth sx={{ bgcolor: LIME, color: INK, fontWeight: 800, borderRadius: '999px', py: 1.4 }}>
                    {plan.cta}
                  </Button>
                ) : (
                  <UpgradeButton tier={plan.tier} label={plan.cta} fullWidth sx={{ bgcolor: dark ? LIME : INK, color: dark ? INK : ALABASTER, fontWeight: 800, borderRadius: '999px', py: 1.4 }} />
                )}
              </Box>
            </Box>
          );
        })}
      </Box>
    </Section>
  );
}

function PricingFAQ() {
  const items = [
    { q: 'What happens when I hit the cost cap?', a: 'The router downgrades to cheaper models first (Haiku, GPT-4o-mini, on-device). If you’re still over, builds pause cleanly with a clear message — never a surprise overage bill.' },
    { q: 'Can I bring my own provider keys?',     a: 'On Team and Enterprise, yes. Plug in your Anthropic / OpenAI key and your routing policy charges against your own quota. Available now in private beta for Pro.' },
    { q: 'Yearly billing?',                       a: 'Yes. Switch the toggle in the calculator to see the 20% yearly discount. Paid up-front, prorated when you upgrade tiers.' },
    { q: 'Refunds?',                              a: '14-day refund on any paid plan, no questions. Cancel from /app/settings — your projects stay readable, and you can re-export the repo at any time.' },
  ];
  return (
    <Section>
      <SectionHeader eyebrow="Pricing FAQ" title="What people ask before they upgrade." />
      <Box sx={{ maxWidth: 900, mx: 'auto' }}>
        {items.map((item) => (
          <Box
            key={item.q}
            component="details"
            sx={{
              borderBottom: '1px solid rgba(13,14,15,0.1)',
              py: 2.2,
              '& > summary': { listStyle: 'none', cursor: 'pointer' },
              '& > summary::-webkit-details-marker': { display: 'none' },
              '&[open] .faq-marker': { transform: 'rotate(45deg)' },
            }}
          >
            <Box component="summary" sx={{ display: 'flex', justifyContent: 'space-between', gap: 2 }}>
              <Typography sx={{ fontSize: { xs: '1.05rem', md: '1.18rem' }, fontWeight: 700, color: INK }}>{item.q}</Typography>
              <Box className="faq-marker" sx={{ width: 20, height: 20, position: 'relative', transition: `transform ${tokens.motion.base} ${tokens.motion.curve}`, flexShrink: 0, '&::before, &::after': { content: '""', position: 'absolute', background: INK, left: '50%', top: '50%', transform: 'translate(-50%, -50%)' }, '&::before': { width: 12, height: 2 }, '&::after': { width: 2, height: 12 } }} />
            </Box>
            <Typography sx={{ mt: 1.4, color: MUTED, fontSize: { xs: '0.95rem', md: '1rem' }, lineHeight: 1.55 }}>{item.a}</Typography>
          </Box>
        ))}
      </Box>
    </Section>
  );
}

// -----------------------------------------------------------------------
// SECURITY PAGE
// -----------------------------------------------------------------------

export function SecurityPage() {
  return (
    <MarketingShell>
      <PageHero
        eyebrow="Security"
        title="Built so the AI never ships an exploit."
        subtitle="Security is a gate, not a paragraph at the bottom of a website. Every patch runs through secret scanning, dep audit, and OWASP-shaped checks before it lands."
      />
      <Section>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)' }, gap: 2 }}>
          {[
            {
              icon: <Shield />,
              title: 'Encryption at rest and in transit',
              text: 'TLS 1.3 everywhere. Database disks encrypted with cloud-managed keys (AWS KMS / GCP CMEK). Backups encrypted with a separate key class.',
            },
            {
              icon: <Lock />,
              title: 'Secret storage',
              text: 'Project secrets live in Kubernetes Secrets you control. Provider keys go through VSCode SecretStorage or your own Vault — never in prompts, never in repo files, never in logs.',
            },
            {
              icon: <VerifiedUser />,
              title: 'Tenant isolation',
              text: 'Per-user Docker sandbox with no shared workspace. Every store has an OwnerID + requireProjectAccess middleware; non-owners get a 404, not a 403, so even existence isn’t leaked.',
            },
            {
              icon: <Whatshot />,
              title: 'Patch lifecycle',
              text: 'The AI never writes files directly. Every change is a patch the engine proposes, gates approve, and the file system applies — auditable and reversible.',
            },
            {
              icon: <Insights />,
              title: 'Data retention',
              text: 'Default 30-day retention on chat history + gate output. Enterprise plans can pin retention to 0 days (write-through to your storage) or up to 7 years for compliance.',
            },
            {
              icon: <Bolt />,
              title: 'AI provider posture',
              text: 'Anthropic + OpenAI configured with zero-retention endpoints where the provider supports it. On-device ONNX option for sensitive flows — nothing leaves your cluster.',
            },
          ].map((item) => (
            <Box key={item.title} sx={{ ...panelSx, p: { xs: 3, md: 4 }, display: 'flex', gap: 2.4, alignItems: 'flex-start' }}>
              <Box sx={{ width: 48, height: 48, borderRadius: '12px', bgcolor: INK, color: LIME, display: 'grid', placeItems: 'center', flexShrink: 0 }}>
                {item.icon}
              </Box>
              <Box>
                <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: { xs: '1.6rem', md: '2rem' }, lineHeight: 1, letterSpacing: 0 }}>
                  {item.title}
                </Typography>
                <Typography variant="body2" sx={{ mt: 1.4, color: MUTED, fontWeight: 500, lineHeight: 1.55 }}>
                  {item.text}
                </Typography>
              </Box>
            </Box>
          ))}
        </Box>
      </Section>
      <Section dark>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 2 }}>
          {[
            { title: 'SOC 2 Type II',         status: 'In progress · Q3 2026', text: 'Under active audit with a top-3 SOC 2 firm. Bridge letter available now under NDA.' },
            { title: 'GDPR + DPA',             status: 'Available',              text: 'Standard DPA, SCCs in place. EU data-residency clusters available on Enterprise.' },
            { title: 'ISO 27001',              status: 'Roadmap · 2027',         text: 'Tracking ISO 27001 for late 2027 once SOC 2 lands. Controls already largely overlap.' },
          ].map((c) => (
            <Box key={c.title} sx={{ borderRadius: 1, p: 3.2, border: '1px solid rgba(244,240,232,0.1)', bgcolor: 'rgba(244,240,232,0.03)' }}>
              <Typography variant="overline" sx={{ color: LIME, fontWeight: 900, letterSpacing: '0.14em' }}>{c.status}</Typography>
              <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: 26, lineHeight: 1, letterSpacing: 0, mt: 0.6 }}>{c.title}</Typography>
              <Typography variant="body2" sx={{ mt: 1.4, color: '#cfc7b8', fontWeight: 500, lineHeight: 1.55 }}>{c.text}</Typography>
            </Box>
          ))}
        </Box>
      </Section>
      <Section id="privacy">
        <SectionHeader eyebrow="Privacy" title="What we store, what we don’t." />
        <Stack spacing={2} sx={{ maxWidth: 900 }}>
          {[
            { what: 'Prompts and chat history',  retention: '30 days by default · configurable on Enterprise' },
            { what: 'Generated code + patches',  retention: 'For the life of the workspace; deleted with the project' },
            { what: 'Provider API responses',    retention: 'Zero-retention where the provider supports it; otherwise 24h for debugging only' },
            { what: 'Billing + cost ledger',     retention: '7 years (financial records)' },
            { what: 'Auth tokens',               retention: 'Bcrypt-hashed; refresh tokens rotated per session' },
            { what: 'Telemetry',                 retention: 'Aggregated only; no per-user code content sent off-cluster' },
          ].map((row) => (
            <Box key={row.what} sx={{ ...panelSx, p: 2.4, display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 2 }}>
              <Typography sx={{ fontWeight: 800, fontSize: 15.5 }}>{row.what}</Typography>
              <Typography sx={{ color: MUTED, fontWeight: 600, fontSize: 14.5 }}>{row.retention}</Typography>
            </Box>
          ))}
        </Stack>
      </Section>
      <FinalCta primary="Security questionnaire" primaryHref="mailto:security@ironflyer.dev" secondary="Talk to engineering" secondaryHref="/enterprise" />
    </MarketingShell>
  );
}

// -----------------------------------------------------------------------
// ENTERPRISE PAGE
// -----------------------------------------------------------------------

export function EnterprisePage() {
  return (
    <MarketingShell>
      <PageHero
        eyebrow="Enterprise"
        title="A finisher loop your security team can sign off on."
        subtitle="Identity, audit, budget caps, private deployment. The platform brings AI speed; your governance keeps it inside the lines."
        primary="Request a demo"
        primaryHref="#enterprise-intake"
        secondary="Read the security brief"
        secondaryHref="/security"
      />
      <Section>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' }, gap: 2 }}>
          {[
            { icon: <VerifiedUser />, title: 'SOC 2 in progress', text: 'Under active SOC 2 Type II audit with a top-3 firm. Bridge letter on request.' },
            { icon: <Lock />,         title: 'On-prem / BYO cloud', text: 'Ship the Helm chart to your Kubernetes cluster. Your data plane, your provider keys.' },
            { icon: <Shield />,       title: 'Dedicated support',   text: 'Named CSM, Slack-shared channel, 99.9% SLA on the orchestrator and runtime.' },
          ].map((c) => (
            <Box key={c.title} sx={{ ...panelSx, p: 3.2 }}>
              <Box sx={{ width: 44, height: 44, borderRadius: '10px', bgcolor: INK, color: LIME, display: 'grid', placeItems: 'center' }}>
                {c.icon}
              </Box>
              <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: 24, lineHeight: 1, mt: 2 }}>{c.title}</Typography>
              <Typography variant="body2" sx={{ mt: 1.2, color: MUTED, fontWeight: 600, lineHeight: 1.55 }}>{c.text}</Typography>
            </Box>
          ))}
        </Box>
      </Section>
      <Section id="enterprise-intake" dark>
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '0.85fr 1.15fr' }, gap: { xs: 3, md: 5 }, alignItems: 'stretch' }}>
          <Box sx={{ p: { xs: 1, md: 2 } }}>
            <Typography variant="overline" sx={{ color: LIME, fontWeight: 900, letterSpacing: '0.14em' }}>Talk to sales</Typography>
            <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: { xs: '2.4rem', md: '3.8rem' }, lineHeight: 0.94, mt: 1.2, color: ALABASTER }}>
              Get a demo and a deployment plan in one conversation.
            </Typography>
            <Typography sx={{ mt: 2.2, color: '#cfc7b8', fontWeight: 500, lineHeight: 1.55, fontSize: { xs: 15, md: 17 } }}>
              We’ll walk through the finisher gates against your own scenario, scope the SSO + audit posture, and put a deployment plan on paper before you commit.
            </Typography>
            <Stack spacing={1.2} sx={{ mt: 3.2 }}>
              {[
                'SSO (SAML / OIDC) + SCIM provisioning',
                'Self-hosted Helm chart on your cluster',
                'Per-team budget caps with finance-visible ledgers',
                'Named CSM, Slack-shared support channel',
                '99.9% SLA on orchestrator and runtime',
              ].map((line) => (
                <Stack key={line} direction="row" spacing={1.2} alignItems="center">
                  <CheckCircle sx={{ color: LIME, fontSize: 18 }} />
                  <Typography variant="body2" sx={{ color: ALABASTER, fontWeight: 700 }}>{line}</Typography>
                </Stack>
              ))}
            </Stack>
          </Box>
          <EnterpriseLeadForm />
        </Box>
      </Section>
      <Section id="careers">
        <SectionHeader eyebrow="Working with us" title="Two ways to join the loop." />
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: 'repeat(2, 1fr)' }, gap: 2 }}>
          {[
            { title: 'Design partner', text: 'Pre-revenue or post-revenue teams who want to shape the roadmap and get founder-level access. Discount in exchange for case-study and feedback cadence.' },
            { title: 'Careers',         text: 'We hire for taste, depth, and shipping speed. If you’ve built systems where correctness matters, send a note to hello@ironflyer.dev.' },
          ].map((c) => (
            <Box key={c.title} sx={{ ...panelSx, p: 3.2 }}>
              <Typography sx={{ fontFamily: tokens.font.display, fontWeight: 400, fontSize: 28, lineHeight: 1 }}>{c.title}</Typography>
              <Typography variant="body2" sx={{ mt: 1.4, color: MUTED, fontWeight: 600, lineHeight: 1.55 }}>{c.text}</Typography>
            </Box>
          ))}
        </Box>
      </Section>
      <FinalCta primary="Request enterprise demo" primaryHref="#enterprise-intake" secondary="Read security brief" secondaryHref="/security" />
    </MarketingShell>
  );
}
