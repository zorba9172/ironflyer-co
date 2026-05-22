// /changelog — release timeline. The voice is plain-English, "we shipped X
// because Y". Categories use small ASCII tokens so the timeline scans even
// without colour.

import type { Metadata } from 'next';
import Link from 'next/link';
import { Box, Chip, Container, Stack, Typography } from '@mui/material';
import { MarketingShellClient } from '../marketing-shell';
import { tokens } from '../../../../packages/design-tokens';

export const metadata: Metadata = {
  title: 'Changelog — Ironflyer',
  description: 'Every shipped change to the Ironflyer AI Product Finisher, newest first.',
  openGraph: {
    title: 'Changelog · Ironflyer',
    description: 'What we shipped and why.',
    images: ['/opengraph-image'],
  },
};

type Category = 'new' | 'improved' | 'fixed' | 'security';

interface Entry {
  version: string;
  date: string;
  headline: string;
  body: string;
  notes: Array<{ category: Category; text: string }>;
}

const CATEGORY_LABEL: Record<Category, string> = {
  new: 'New',
  improved: 'Improved',
  fixed: 'Fixed',
  security: 'Security',
};

const CATEGORY_COLOR: Record<Category, string> = {
  new: '#671dfc',
  improved: '#5c6300',
  fixed: '#ff6c3a',
  security: '#ff1818',
};

const ENTRIES: Entry[] = [
  {
    version: 'v0.10.0',
    date: '2026-05-12',
    headline: 'The finisher loop, end-to-end.',
    body:
      'The full Planner → Coder → Patch → Apply → Gates loop is now wired in production. Every gate emits structured issues, every patch goes through the security scan, and the RuntimeApplier mirrors changes into your sandbox so the live preview reflects reality on the next reload. Auto-recovery is the bigger story: when a gate goes red, a scoped recovery agent now proposes a targeted patch instead of restarting the loop from scratch — average recovery time fell from ~3 minutes to under 40 seconds in our internal runs.',
    notes: [
      { category: 'new', text: 'Auto-recovery agent for failed gates, scoped to the gate that failed.' },
      { category: 'new', text: 'RuntimeApplier mirrors applied patches into the sandbox so the preview reloads cleanly.' },
      { category: 'improved', text: 'Lint + Test gates now stream issues as they appear instead of waiting for the run to finish.' },
      { category: 'fixed', text: 'Coder agent no longer re-emits a no-op patch when the file content already matches.' },
    ],
  },
  {
    version: 'v0.9.5',
    date: '2026-04-22',
    headline: 'Live preview proxy with WebSocket upgrade.',
    body:
      'Preview now supports WebSocket upgrades, so Next.js HMR, Vite HMR, and any framework with a dev socket work cleanly inside the iframe. The proxy is signed per session — no auth header is needed in the iframe, which is exactly what makes the preview work inside the VSCode webview. Tokens rotate every 24 hours.',
    notes: [
      { category: 'new', text: 'WebSocket upgrade support in the preview proxy.' },
      { category: 'improved', text: 'Preview tokens rotate every 24 hours and survive page reloads.' },
      { category: 'fixed', text: 'Preview iframe no longer flashes a 404 when the sandbox is still warming up.' },
    ],
  },
  {
    version: 'v0.9.0',
    date: '2026-04-08',
    headline: 'Cloud workspace v1.',
    body:
      'Every project now boots a real Linux sandbox on the workspace runtime. The file system in the sandbox is the source of truth — what you see in the file tree, what the test gate runs against, what the live preview serves. The runtime supports two drivers (Mock for dev, Docker for production) and exposes file I/O, exec, PTY over WebSocket, and a port API for the preview proxy.',
    notes: [
      { category: 'new', text: 'Per-user Linux sandboxes managed by the workspace runtime.' },
      { category: 'new', text: 'PTY over WebSocket — works with xterm.js out of the box.' },
      { category: 'improved', text: 'File operations now go through a dedicated API surface instead of the orchestrator.' },
    ],
  },
  {
    version: 'v0.8.5',
    date: '2026-03-19',
    headline: 'One-click deploy to Fly and Railway, plus GitHub export.',
    body:
      'The Deploy gate now generates the artifacts your provider wants — Dockerfile, fly.toml, railway.json, a workflow file for the GitHub export — and proposes them as a patch. Hitting Deploy from the dashboard pushes to Fly or Railway and returns a live URL. The GitHub exporter creates (or updates) a repo under your account and pushes the project tree.',
    notes: [
      { category: 'new', text: 'POST /projects/{id}/deploy with provider=fly|railway and streamed SSE log.' },
      { category: 'new', text: 'POST /projects/{id}/export/github creates a repo and pushes the project files.' },
      { category: 'new', text: 'POST /projects/{id}/export/zip streams a packaged project archive.' },
      { category: 'improved', text: 'Generated deploy artifacts now go through the Security gate before they ship.' },
    ],
  },
  {
    version: 'v0.8.0',
    date: '2026-03-02',
    headline: 'VSCode extension v0.3.0 with live preview webview.',
    body:
      'The VSCode extension is no longer just a chat panel. The 0.3 release ships a webview-based live preview, a finisher-gates tree, a patches tree with diff opening, an output channel for run events, and a quick-action that turns any editor diagnostic into a coder-targeted chat prompt. Auth lives in SecretStorage; the URI handler picks up the JWT from the web sign-in flow.',
    notes: [
      { category: 'new', text: 'Live preview webview inside the editor, with mobile / tablet / desktop presets.' },
      { category: 'new', text: 'Patches tree — open in VSCode diff editor, apply through the orchestrator.' },
      { category: 'new', text: 'Quick action: turn an editor diagnostic into a coder-targeted chat prompt.' },
      { category: 'improved', text: 'Status bar shows the pinned project, last gate status, and budget remaining.' },
    ],
  },
  {
    version: 'v0.7.0',
    date: '2026-02-11',
    headline: 'Budget transparency dashboard.',
    body:
      'The new budget dashboard shows your lifetime spend, your plan cap, and the most recent ledger entries. The global vault snapshot — revenue, provider cost, margin — is now public at GET /budget/vault. We built this because most AI app builders hide the unit economics behind a marketing claim; we would rather show the math.',
    notes: [
      { category: 'new', text: 'Dashboard surface for the per-user vault snapshot.' },
      { category: 'new', text: 'Public GET /budget/vault and /budget/plans + /budget/rates endpoints.' },
      { category: 'improved', text: 'BillingGuard now charges only after a stream completes — the margin number is the closed books.' },
    ],
  },
  {
    version: 'v0.6.0',
    date: '2026-01-28',
    headline: 'Brand refresh: alabaster + lime.',
    body:
      'We retired the green-on-black look and adopted an alabaster + electric-lime palette inspired by output.com. The lime is the single accent — used only on the primary CTA, the live status pip, and the active TOC entry. Display type moved to Archivo Black; body type stays on Inter. Everything else got a generous helping of whitespace.',
    notes: [
      { category: 'new', text: 'design-tokens package with output.com-inspired aesthetic.' },
      { category: 'improved', text: 'Marketing surface rebuilt with MUI 6 and the new tokens.' },
      { category: 'improved', text: 'Hebrew copy promoted to first-class on every marketing page.' },
    ],
  },
  {
    version: 'v0.5.0',
    date: '2026-01-08',
    headline: 'Public beta launch.',
    body:
      'We opened sign-ups. Free tier gives you four projects and ~50 finisher runs a month; paid plans start at $20 with a hard cost cap that defines our margin floor. The public beta ships with the orchestrator (auth, projects, gates, patches, budget), the workspace runtime in mock-driver mode, the Next.js dashboard, and the SDK.',
    notes: [
      { category: 'new', text: 'Free and Pro plans available at signup; Team and Enterprise on request.' },
      { category: 'new', text: 'Stripe checkout for plan upgrades + webhook for plan changes.' },
      { category: 'security', text: 'Every project route is owner-scoped; non-owners get 404 to avoid leaking existence.' },
    ],
  },
];

export default function ChangelogPage() {
  return (
    <MarketingShellClient>
      <Box sx={{ bgcolor: tokens.color.bg.alabaster, minHeight: '100vh' }}>
        <Container maxWidth="md" sx={{ py: { xs: 6, md: 10 } }}>
          <Stack direction="row" alignItems="baseline" justifyContent="space-between" sx={{ mb: 1 }}>
            <Typography variant="overline" sx={{ color: '#5c5750', letterSpacing: '0.16em', fontWeight: 800, fontSize: 12 }}>
              Changelog · יומן שינויים
            </Typography>
            <Box
              component="a"
              href="/changelog/rss.xml"
              sx={{
                fontSize: 12,
                color: '#5c6300',
                fontWeight: 700,
                textDecoration: 'none',
                borderBottom: '1px dashed currentColor',
                '&:hover': { color: '#3a4000' },
              }}
            >
              RSS
            </Box>
          </Stack>
          <Typography
            component="h1"
            sx={{
              fontFamily: tokens.font.display,
              fontSize: { xs: 42, md: 58 },
              lineHeight: 1.04,
              color: '#0d0e0f',
              mb: 1.5,
            }}
          >
            What we shipped.
          </Typography>
          <Typography sx={{ color: '#3a3530', fontSize: 18, lineHeight: 1.6, maxWidth: 640, mb: 6 }}>
            Every release of the Ironflyer AI Product Finisher, newest first. We try to write these the
            way we would explain a change in a code review — what we did, and why.
          </Typography>

          <Box>
            {ENTRIES.map((entry, idx) => (
              <Box
                key={entry.version}
                component="article"
                sx={{
                  position: 'relative',
                  pl: { xs: 0, md: 5 },
                  pb: 6,
                  mb: 6,
                  borderBottom: idx < ENTRIES.length - 1 ? '1px solid rgba(17,17,17,0.10)' : 'none',
                }}
              >
                <Box
                  sx={{
                    display: { xs: 'none', md: 'block' },
                    position: 'absolute',
                    left: 0,
                    top: 8,
                    width: 12,
                    height: 12,
                    borderRadius: '999px',
                    bgcolor: idx === 0 ? tokens.color.accent.lime : '#d6cfc1',
                    boxShadow: idx === 0 ? '0 0 14px rgba(229,255,0,0.55)' : 'none',
                  }}
                />
                <Stack direction="row" alignItems="baseline" spacing={2} sx={{ mb: 1 }}>
                  <Typography sx={{ fontFamily: tokens.font.mono, fontSize: 13.5, fontWeight: 800, color: '#0d0e0f' }}>
                    {entry.version}
                  </Typography>
                  <Typography sx={{ color: '#77736b', fontSize: 13 }}>{entry.date}</Typography>
                </Stack>
                <Typography
                  component="h2"
                  sx={{
                    fontFamily: tokens.font.display,
                    fontSize: { xs: 26, md: 30 },
                    lineHeight: 1.15,
                    color: '#0d0e0f',
                    mb: 1.4,
                  }}
                >
                  {entry.headline}
                </Typography>
                <Typography sx={{ color: '#262320', fontSize: 16, lineHeight: 1.75, mb: 2.4 }}>
                  {entry.body}
                </Typography>
                <Stack spacing={1.2}>
                  {entry.notes.map((n, i) => (
                    <Stack key={i} direction="row" spacing={1.4} alignItems="flex-start">
                      <Chip
                        label={CATEGORY_LABEL[n.category]}
                        size="small"
                        sx={{
                          bgcolor: CATEGORY_COLOR[n.category],
                          color: '#fff',
                          fontWeight: 800,
                          fontSize: 10.5,
                          letterSpacing: '0.06em',
                          textTransform: 'uppercase',
                          height: 22,
                          minWidth: 76,
                        }}
                      />
                      <Typography sx={{ color: '#262320', fontSize: 15, lineHeight: 1.65 }}>{n.text}</Typography>
                    </Stack>
                  ))}
                </Stack>
              </Box>
            ))}
          </Box>

          <Box sx={{ mt: 4, p: 3, borderRadius: 2.5, border: '1px solid rgba(17,17,17,0.10)', bgcolor: '#fbf8f1' }}>
            <Typography sx={{ color: '#3a3530', fontSize: 14, lineHeight: 1.65 }}>
              Want to follow along? Subscribe via <Link href="/changelog/rss.xml" style={{ color: '#5c6300', fontWeight: 700 }}>RSS</Link>,
              or read our deep-dives on the <Link href="/blog" style={{ color: '#5c6300', fontWeight: 700 }}>blog</Link>.
            </Typography>
          </Box>
        </Container>
      </Box>
    </MarketingShellClient>
  );
}
