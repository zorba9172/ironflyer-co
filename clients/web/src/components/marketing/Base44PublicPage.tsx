"use client";

import {
  AccountTreeRounded,
  ArrowForwardRounded,
  AutoAwesomeRounded,
  BoltRounded,
  CheckCircleRounded,
  CodeRounded,
  ExtensionRounded,
  GroupsRounded,
  HubRounded,
  IntegrationInstructionsRounded,
  LockRounded,
  RocketLaunchRounded,
  SchoolRounded,
  ShieldRounded,
  StorefrontRounded,
  TerminalRounded,
  TuneRounded,
  ViewKanbanRounded,
} from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import type { ReactNode } from "react";
import { tokens } from "../../theme";

type PublicPageKind =
  | "templates"
  | "solutions"
  | "pricing"
  | "resources"
  | "enterprise"
  | "vscode"
  | "appsec";

type Tone = "light" | "dark";

interface PageData {
  eyebrow: string;
  title: string;
  accent: string;
  subhead: string;
  primary: string;
  secondary: string;
  primaryHref?: string;
  secondaryHref?: string;
  chips: string[];
  sections: Array<{
    eyebrow: string;
    title: string;
    subhead: string;
    items: Array<{
      title: string;
      body: string;
      icon: ReactNode;
      meta?: string;
    }>;
  }>;
  quote?: string;
}

const pageData: Record<PublicPageKind, PageData> = {
  templates: {
    eyebrow: "Templates",
    title: "Start closer to done.",
    accent: "Pick a proven app shape and make it yours.",
    subhead:
      "Choose a foundation for a portal, dashboard, marketplace, internal tool or mobile product. IronFlyer opens it in Studio with the prompt, data model and first screens already lined up.",
    primary: "Use a template",
    secondary: "Open Studio",
    primaryHref: "/studio",
    secondaryHref: "/studio",
    chips: [
      "Client portals",
      "SaaS dashboards",
      "Marketplaces",
      "Admin tools",
      "Mobile apps",
    ],
    sections: [
      {
        eyebrow: "popular starts",
        title: "Five templates teams reach for first.",
        subhead: "Each template is a product pattern, not a blank canvas.",
        items: [
          {
            title: "SaaS Starter",
            body: "Auth, billing, teams, roles and product analytics.",
            icon: <BoltRounded />,
            meta: "92 score",
          },
          {
            title: "Client Portal",
            body: "Projects, files, approvals, invoices and activity.",
            icon: <GroupsRounded />,
            meta: "88 score",
          },
          {
            title: "Marketplace",
            body: "Listings, search, checkout and seller workflows.",
            icon: <StorefrontRounded />,
            meta: "94 score",
          },
          {
            title: "Internal Tool",
            body: "Approvals, reporting, admin tables and permissions.",
            icon: <ViewKanbanRounded />,
            meta: "91 score",
          },
          {
            title: "Education App",
            body: "Lessons, progress, cohorts and analytics.",
            icon: <SchoolRounded />,
            meta: "86 score",
          },
          {
            title: "Mobile Companion",
            body: "Onboarding, profiles, notifications and synced data.",
            icon: <RocketLaunchRounded />,
            meta: "89 score",
          },
        ],
      },
      {
        eyebrow: "how it works",
        title: "Template to working app in one pass.",
        subhead:
          "Pick, refine, generate, review and publish without leaving the same workspace.",
        items: [
          {
            title: "Pick",
            body: "Start from a template that matches the job.",
            icon: <CheckCircleRounded />,
          },
          {
            title: "Refine",
            body: "Tell Studio what to change before the build runs.",
            icon: <TuneRounded />,
          },
          {
            title: "Generate",
            body: "Screens, data and code are created together.",
            icon: <AutoAwesomeRounded />,
          },
          {
            title: "Ship",
            body: "Preview, test and publish from the same lane.",
            icon: <RocketLaunchRounded />,
          },
        ],
      },
    ],
    quote: "The fastest build is the one that starts with the right shape.",
  },
  solutions: {
    eyebrow: "Solutions",
    title: "Build the app your team actually needs.",
    accent: "From internal tools to customer products.",
    subhead:
      "Describe the workflow, choose the target, and let IronFlyer turn it into a working product with auth, data, UI, preview and deploy paths included.",
    primary: "Start building",
    secondary: "Browse templates",
    primaryHref: "/studio",
    secondaryHref: "/templates",
    chips: [
      "Operations",
      "Sales",
      "Product",
      "Education",
      "Finance",
      "Dev teams",
    ],
    sections: [
      {
        eyebrow: "use cases",
        title: "Common products, ready to adapt.",
        subhead:
          "Prompt in plain English. Keep control as the product takes shape.",
        items: [
          {
            title: "Customer portals",
            body: "Accounts, files, billing, approvals and support flows.",
            icon: <GroupsRounded />,
          },
          {
            title: "Internal operations",
            body: "Dashboards, queues, admin workflows and role-based access.",
            icon: <ViewKanbanRounded />,
          },
          {
            title: "Sales tools",
            body: "CRM flows, lead intake, pipeline views and follow-up reminders.",
            icon: <StorefrontRounded />,
          },
          {
            title: "Analytics apps",
            body: "Connected data, charts, filters and reporting screens.",
            icon: <AccountTreeRounded />,
          },
          {
            title: "Mobile products",
            body: "Responsive screens, mobile previews and deploy checklists.",
            icon: <RocketLaunchRounded />,
          },
          {
            title: "Developer workflows",
            body: "VS Code handoff, patch review and generated code you own.",
            icon: <CodeRounded />,
          },
        ],
      },
      {
        eyebrow: "included",
        title: "The basics are not extra work.",
        subhead:
          "Every generated product starts with the pieces real teams ask for.",
        items: [
          {
            title: "Auth and roles",
            body: "Sign in, teams, permissions and account boundaries.",
            icon: <LockRounded />,
          },
          {
            title: "Data models",
            body: "Tables, relations, forms and useful starter records.",
            icon: <HubRounded />,
          },
          {
            title: "Integrations",
            body: "Connect APIs, webhooks and services when the app needs them.",
            icon: <IntegrationInstructionsRounded />,
          },
          {
            title: "Launch path",
            body: "Preview, iterate, test and deploy from one workspace.",
            icon: <RocketLaunchRounded />,
          },
        ],
      },
    ],
    quote: "The product should feel clear before the code feels clever.",
  },
  pricing: {
    eyebrow: "Pricing",
    title: "Start with a plan. Pay when the build runs.",
    accent: "Clear workspace plans with wallet controls for generation work.",
    subhead:
      "Shape ideas for free, then fund paid executions with a wallet hold you approve before code is generated. See how each plan compares to Lovable, Bolt, Base44 and v0 on the comparison page.",
    primary: "Start free",
    secondary: "Compare vs others",
    primaryHref: "/studio",
    secondaryHref: "/compare",
    chips: [
      "Plan first",
      "Wallet controls",
      "Budget holds",
      "Private projects",
      "Team roles",
    ],
    sections: [
      {
        eyebrow: "plans",
        title: "Pick the workspace level, then control each build.",
        subhead:
          "No hidden execution starts. Every paid run has an explicit budget.",
        items: [
          {
            title: "Free",
            body: "$0 workspace access for shaping prompts, reviewing flows and starting small.",
            icon: <CheckCircleRounded />,
            meta: "$0",
          },
          {
            title: "Pro",
            body: "More private projects, exports and founder support. Executions use wallet credits.",
            icon: <BoltRounded />,
            meta: "$29/mo",
          },
          {
            title: "Team",
            body: "Roles, shared workspaces, environments and priority launch support.",
            icon: <GroupsRounded />,
            meta: "$79/mo",
          },
          {
            title: "Enterprise",
            body: "SSO, audit logs, private deployment and custom integrations.",
            icon: <ShieldRounded />,
            meta: "Custom",
          },
        ],
      },
      {
        eyebrow: "why it is clear",
        title: "You always know what happens next.",
        subhead:
          "Budgets, plan-first mode and deploy controls are visible before the build runs.",
        items: [
          {
            title: "Plan first",
            body: "Review the shape before the app is generated.",
            icon: <TuneRounded />,
          },
          {
            title: "Budget controls",
            body: "Approve the wallet hold before generation starts.",
            icon: <CheckCircleRounded />,
          },
          {
            title: "Code ownership",
            body: "Export and keep the generated React and TypeScript.",
            icon: <CodeRounded />,
          },
          {
            title: "Security",
            body: "Roles, auth and deploy gates are part of the workflow.",
            icon: <ShieldRounded />,
          },
        ],
      },
    ],
    quote: "A pricing page should answer the next decision, not hide it.",
  },
  resources: {
    eyebrow: "Resources",
    title: "Learn the product path before you build.",
    accent: "Guides, patterns and answers for shipping with confidence.",
    subhead:
      "Use the library to choose a template, sharpen a prompt, understand integrations and prepare the app for launch.",
    primary: "Open Studio",
    secondary: "View templates",
    primaryHref: "/studio",
    secondaryHref: "/templates",
    chips: [
      "Guides",
      "Prompt examples",
      "Integrations",
      "Launch checklists",
      "Help",
    ],
    sections: [
      {
        eyebrow: "library",
        title: "The right help at the right stage.",
        subhead: "No filler. Just the decisions that move a build forward.",
        items: [
          {
            title: "Prompt guides",
            body: "Turn a loose idea into a usable product brief.",
            icon: <AutoAwesomeRounded />,
          },
          {
            title: "Template notes",
            body: "Know when to start from SaaS, portal, marketplace or admin.",
            icon: <ViewKanbanRounded />,
          },
          {
            title: "Integration playbooks",
            body: "Connect payments, email, data sources and webhooks.",
            icon: <IntegrationInstructionsRounded />,
          },
          {
            title: "Launch checklist",
            body: "Review auth, roles, mobile, deploy and rollback.",
            icon: <CheckCircleRounded />,
          },
        ],
      },
      {
        eyebrow: "support",
        title: "When you need a human, ask for one.",
        subhead:
          "The product stays self-serve, but serious launches deserve clear support.",
        items: [
          {
            title: "Docs",
            body: "Short explanations for the workflow you are in.",
            icon: <CodeRounded />,
          },
          {
            title: "Examples",
            body: "Use cases and prompts you can adapt directly.",
            icon: <SchoolRounded />,
          },
          {
            title: "Partners",
            body: "Bring in help for a larger launch or migration.",
            icon: <GroupsRounded />,
          },
          {
            title: "Enterprise review",
            body: "Security, SSO and private deployment support.",
            icon: <ShieldRounded />,
          },
        ],
      },
    ],
    quote: "Good docs make the product feel smaller in the best way.",
  },
  enterprise: {
    eyebrow: "Enterprise",
    title: "Bring prompt-built apps into your company safely.",
    accent: "Security, control and team workflows built in.",
    subhead:
      "IronFlyer gives teams SSO, audit logs, private deployments, role controls and generated code that can move through real engineering review.",
    primary: "Contact sales",
    secondary: "View security",
    primaryHref: "mailto:founder@ironflyer.dev?subject=Enterprise%20intro",
    secondaryHref: "/resources",
    chips: [
      "SSO",
      "RBAC",
      "Audit logs",
      "Private deploy",
      "No training on code",
    ],
    sections: [
      {
        eyebrow: "controls",
        title: "Everything security asks for first.",
        subhead:
          "The app builder is fast. The controls around it are deliberate.",
        items: [
          {
            title: "Single sign-on",
            body: "Connect identity and keep access under company policy.",
            icon: <LockRounded />,
          },
          {
            title: "Role-based access",
            body: "Control who can view, build, review and deploy.",
            icon: <GroupsRounded />,
          },
          {
            title: "Audit history",
            body: "Track prompts, generated changes, approvals and deploys.",
            icon: <CheckCircleRounded />,
          },
          {
            title: "Private deployment",
            body: "Run apps in the environment your team trusts.",
            icon: <ShieldRounded />,
          },
          {
            title: "Code ownership",
            body: "Generated code can be reviewed, exported and governed.",
            icon: <CodeRounded />,
          },
          {
            title: "Data boundaries",
            body: "Customer prompts and code are never used for training.",
            icon: <LockRounded />,
          },
        ],
      },
      {
        eyebrow: "rollout",
        title: "Adopt it without breaking your process.",
        subhead: "Start with a real app, then expand to teams and controls.",
        items: [
          {
            title: "Pilot",
            body: "Pick one internal workflow and build it end to end.",
            icon: <RocketLaunchRounded />,
          },
          {
            title: "Review",
            body: "Validate generated code, security and deploy behavior.",
            icon: <ShieldRounded />,
          },
          {
            title: "Enable teams",
            body: "Add roles, templates and shared workspaces.",
            icon: <GroupsRounded />,
          },
          {
            title: "Scale",
            body: "Move repeated workflows into reusable templates.",
            icon: <ViewKanbanRounded />,
          },
        ],
      },
    ],
    quote: "Fast only matters when the company can actually use what ships.",
  },
  vscode: {
    eyebrow: "VS Code Extension",
    title: "IronFlyer for VS Code.",
    accent: "Review patches without leaving your editor.",
    subhead:
      "Pin a project, inspect generated patches, open diffs, ask for fixes and keep the launch loop close to the code.",
    primary: "Get the extension",
    secondary: "Open Studio",
    primaryHref: "/signup?redirect=/studio",
    secondaryHref: "/studio",
    chips: [
      "Patch review",
      "Live gates",
      "Project preview",
      "Secure sign-in",
      "Quick fixes",
    ],
    sections: [
      {
        eyebrow: "editor workflow",
        title: "A serious builder needs more than a browser tab.",
        subhead:
          "Use Studio for the big picture. Use VS Code when the code matters.",
        items: [
          {
            title: "Project tree",
            body: "Pin a project and keep files, patches and gates in view.",
            icon: <ViewKanbanRounded />,
          },
          {
            title: "Patch diffs",
            body: "Open proposed changes in native side-by-side diffs.",
            icon: <CodeRounded />,
          },
          {
            title: "Ask to fix",
            body: "Send diagnostics and snippets back to the coding agent.",
            icon: <AutoAwesomeRounded />,
          },
          {
            title: "Live status",
            body: "Watch run output, budgets, gates and deploy state update.",
            icon: <TerminalRounded />,
          },
        ],
      },
      {
        eyebrow: "why it wins",
        title: "Prompt-first, code-aware.",
        subhead:
          "Base prompt builders stop at the browser. IronFlyer keeps the loop open in the editor.",
        items: [
          {
            title: "Secret storage",
            body: "Auth token is stored in VS Code SecretStorage.",
            icon: <LockRounded />,
          },
          {
            title: "Open VSX path",
            body: "Built for VS Code and compatible editor installs.",
            icon: <ExtensionRounded />,
          },
          {
            title: "Generated code you own",
            body: "Inspect, accept, reject and export changes.",
            icon: <CodeRounded />,
          },
          {
            title: "Studio handoff",
            body: "Jump between the product canvas and code review.",
            icon: <HubRounded />,
          },
        ],
      },
    ],
    quote: "The best AI builder still respects the editor.",
  },
  appsec: {
    eyebrow: "AppSec",
    title: "Every AI-generated change passes through real security gates.",
    accent:
      "Semgrep, gitleaks, trufflehog and govulncheck on every iteration. Critical findings block the deploy lane — they don't sit on a backlog.",
    subhead:
      "Lovable, Bolt, Base44 and v0 ship the prompt and the preview. Ironflyer ships the prompt, the preview, and the security verdict that says whether the change is allowed to leave the workspace. See the full feature comparison.",
    primary: "Open Studio",
    secondary: "Compare vs others",
    primaryHref: "/studio",
    secondaryHref: "/compare",
    chips: [
      "Semgrep SAST",
      "Secret scanning",
      "CVE checks",
      "SOC2 gate",
      "HIPAA gate",
    ],
    sections: [
      {
        eyebrow: "scanners that run",
        title: "Four scanners, baked into every workspace.",
        subhead:
          "The runtime image ships with the binaries pre-installed. The SecurityGate runs them on every iteration and feeds findings back into the repair loop.",
        items: [
          {
            title: "Semgrep",
            body: "OWASP-Top-10 SAST. Maps findings to severity, file and line, then opens a patch.",
            icon: <ShieldRounded />,
            meta: "every iteration",
          },
          {
            title: "gitleaks",
            body: "Secrets in the working tree and history. Critical severity — the deploy lane refuses to proceed.",
            icon: <LockRounded />,
            meta: "blocks deploy",
          },
          {
            title: "trufflehog",
            body: "Entropy plus live provider verification. Catches secrets the regex scanners miss.",
            icon: <LockRounded />,
            meta: "verified secrets",
          },
          {
            title: "govulncheck",
            body: "Real CVE checks against module imports — only paths actually reachable get flagged.",
            icon: <ShieldRounded />,
            meta: "reachable CVEs",
          },
        ],
      },
      {
        eyebrow: "compliance gates",
        title: "SOC2 + HIPAA gates ship in the codebase.",
        subhead:
          "Most AI builders treat compliance as a sales conversation. Ironflyer's compliance gates run automatically when the project spec opts in.",
        items: [
          {
            title: "SOC2 CC6/CC7/CC8",
            body: "Auth declaration, HTTPS binding, audit log, monitoring tool. Each missing artefact opens a finding.",
            icon: <ShieldRounded />,
            meta: "Team tier",
          },
          {
            title: "HIPAA 164.312",
            body: "Access control, audit, integrity, transmission security, PHI tagging. Missing controls block ship.",
            icon: <ShieldRounded />,
            meta: "Team tier",
          },
          {
            title: "Mobile MASVS",
            body: "Android keystore + iOS keychain checks for mobile builds, layered on top of the generic SecurityGate.",
            icon: <ShieldRounded />,
            meta: "Pro tier",
          },
          {
            title: "iOS Privacy Manifest",
            body: "PrivacyInfo.xcprivacy enforcement so App Store review doesn't reject the binary.",
            icon: <ShieldRounded />,
            meta: "Pro tier",
          },
        ],
      },
      {
        eyebrow: "what gates actually do",
        title: "Findings open patches. Patches go through review.",
        subhead:
          "A finding is not a backlog item. The repair agent opens a patch that fixes the issue, the patch is reviewable, and only an approved patch lands.",
        items: [
          {
            title: "Find",
            body: "Scanner emits a finding with file, line, rule and remediation hint.",
            icon: <CheckCircleRounded />,
          },
          {
            title: "Repair",
            body: "RoleSecurity agent proposes a concrete patch — not just a comment.",
            icon: <AutoAwesomeRounded />,
          },
          {
            title: "Review",
            body: "Patch surface shows diff, gate verdict and ProfitGuard reservation.",
            icon: <CodeRounded />,
          },
          {
            title: "Ship",
            body: "Deploy lane checks remaining critical findings — zero means green.",
            icon: <RocketLaunchRounded />,
          },
        ],
      },
    ],
    quote:
      "The competitors ship the prompt. Ironflyer ships the prompt plus the verdict.",
  },
};

function palette(mode: Tone) {
  const light = mode === "light";
  return {
    light,
    bg: light ? "#fbfaff" : tokens.color.bg.base,
    text: light ? "#080b3f" : tokens.color.text.primary,
    secondary: light ? "#5d6588" : tokens.color.text.secondary,
    muted: light ? "#8087a4" : tokens.color.text.muted,
    surface: light ? "rgba(255,255,255,0.78)" : "rgba(16,18,44,0.72)",
    surfaceStrong: light ? "#ffffff" : tokens.color.bg.surfaceRaised,
    border: light ? "rgba(127,77,255,0.18)" : tokens.color.border.subtle,
    strong: light ? "rgba(177,91,255,0.36)" : tokens.color.border.strong,
    wash: light
      ? "radial-gradient(780px 420px at 82% 10%, rgba(231,77,202,0.12), transparent 72%), radial-gradient(760px 380px at 8% 22%, rgba(139,77,255,0.10), transparent 70%)"
      : "radial-gradient(780px 420px at 82% 10%, rgba(177,91,255,0.20), transparent 72%), radial-gradient(760px 380px at 8% 22%, rgba(37,112,255,0.12), transparent 70%)",
  };
}

function PublicHeroObject({ mode }: { mode: Tone }) {
  const light = mode === "light";
  return (
    <Box
      aria-hidden
      sx={{
        display: { xs: "none", md: "block" },
        position: "relative",
        width: 420,
        height: 132,
        mt: 1.2,
        perspective: "1000px",
      }}
    >
      <Box
        sx={{
          position: "absolute",
          left: "50%",
          top: "54%",
          width: 300,
          height: 88,
          borderRadius: "50%",
          border: light
            ? "1px solid rgba(156,91,255,0.20)"
            : "1px solid rgba(170,108,255,0.28)",
          transform: "translate(-50%, -50%) rotateX(68deg) rotateZ(-7deg)",
          boxShadow: light
            ? "0 0 70px rgba(181,91,255,0.11)"
            : "0 0 90px rgba(126,81,255,0.22)",
        }}
      />
      {[0, 1, 2].map((i) => (
        <Box
          key={i}
          sx={{
            position: "absolute",
            left: `${96 + i * 72}px`,
            top: `${22 + (i % 2) * 24}px`,
            width: 86,
            height: 58,
            borderRadius: 2,
            border: light
              ? "1px solid rgba(127,77,255,0.18)"
              : "1px solid rgba(160,113,255,0.28)",
            bgcolor: light ? "rgba(255,255,255,0.70)" : "rgba(14,16,45,0.70)",
            backdropFilter: "blur(16px)",
            transform: `rotateY(${-18 + i * 18}deg) rotateZ(${-8 + i * 5}deg)`,
            boxShadow: light
              ? "0 18px 52px rgba(103,65,180,0.10)"
              : "0 20px 70px rgba(0,0,0,0.25)",
            "&::before, &::after": {
              content: '""',
              position: "absolute",
              left: 12,
              height: 7,
              borderRadius: 999,
              background: `linear-gradient(90deg, ${tokens.color.accent.coral}, ${tokens.color.accent.violet})`,
            },
            "&::before": { top: 18, right: 16 },
            "&::after": { top: 32, right: 32, opacity: 0.55 },
          }}
        />
      ))}
    </Box>
  );
}

export function Base44PublicPage({ page }: { page: PublicPageKind }) {
  const search = useSearchParams();
  const mode: Tone = search?.get("theme") === "dark" ? "dark" : "light";
  const p = palette(mode);
  const data = pageData[page];
  const withTheme = (href: string) =>
    href.startsWith("/")
      ? `${href}${href.includes("?") ? "&" : "?"}theme=${mode}`
      : href;

  return (
    <Box
      sx={{
        bgcolor: p.bg,
        backgroundImage: p.wash,
        color: p.text,
        minHeight: "100vh",
        overflow: "clip",
        px: { xs: 2, md: 4 },
        py: { xs: 6, md: 8 },
      }}
    >
      <Stack spacing={{ xs: 5, md: 7 }} sx={{ maxWidth: 1280, mx: "auto" }}>
        <Stack spacing={2.2} alignItems="center" textAlign="center">
          <Box
            sx={{
              border: `1px solid ${p.border}`,
              borderRadius: 999,
              color: tokens.color.accent.violet,
              fontSize: 14,
              fontWeight: 900,
              px: 2,
              py: 0.8,
              bgcolor: p.surface,
            }}
          >
            {data.eyebrow}
          </Box>
          <Typography
            component="h1"
            sx={{
              fontSize: {
                xs: 36,
                md: page === "appsec" || page === "pricing" ? 62 : 68,
              },
              fontWeight: 950,
              letterSpacing: 0,
              lineHeight: 1.02,
              maxWidth: page === "vscode" ? 900 : 940,
            }}
          >
            {data.title}
            <Box
              component="span"
              sx={{
                display: "block",
                mt: 0.6,
                background: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 54%, ${tokens.color.accent.violet})`,
                backgroundClip: "text",
                color: "transparent",
              }}
            >
              {data.accent}
            </Box>
          </Typography>
          <Typography
            sx={{
              color: p.secondary,
              fontSize: { xs: 17, md: 21 },
              lineHeight: 1.55,
              maxWidth: 760,
            }}
          >
            {data.subhead}
          </Typography>
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={1.4}
            sx={{ pt: 1 }}
          >
            <Button
              component={Link}
              href={withTheme(data.primaryHref ?? "/studio")}
              variant="contained"
              endIcon={<ArrowForwardRounded />}
              sx={{
                minHeight: 52,
                px: 3,
                fontWeight: 900,
                background: `linear-gradient(100deg, ${tokens.color.accent.coral}, ${tokens.color.brand.magenta} 52%, ${tokens.color.accent.violet})`,
              }}
            >
              {data.primary}
            </Button>
            <Button
              component={Link}
              href={withTheme(data.secondaryHref ?? "/templates")}
              variant="outlined"
              sx={{
                minHeight: 52,
                px: 3,
                color: p.text,
                borderColor: p.strong,
                fontWeight: 900,
              }}
            >
              {data.secondary}
            </Button>
          </Stack>
          <Stack
            direction="row"
            useFlexGap
            flexWrap="wrap"
            justifyContent="center"
            sx={{ gap: 1, pt: 1.2 }}
          >
            {data.chips.map((chip) => (
              <Box
                key={chip}
                sx={{
                  border: `1px solid ${p.border}`,
                  borderRadius: 999,
                  bgcolor: p.surface,
                  color: p.secondary,
                  fontSize: 13,
                  fontWeight: 800,
                  px: 1.55,
                  py: 0.7,
                }}
              >
                {chip}
              </Box>
            ))}
          </Stack>
          <PublicHeroObject mode={mode} />
        </Stack>

        {data.sections.map((section) => (
          <Box key={section.eyebrow}>
            <Stack
              spacing={1}
              alignItems="center"
              textAlign="center"
              sx={{ mb: 3 }}
            >
              <Typography
                sx={{
                  color: tokens.color.accent.violet,
                  fontSize: 12,
                  fontWeight: 950,
                  textTransform: "uppercase",
                }}
              >
                {section.eyebrow}
              </Typography>
              <Typography
                sx={{
                  fontSize: { xs: 28, md: 40 },
                  fontWeight: 950,
                  lineHeight: 1.08,
                }}
              >
                {section.title}
              </Typography>
              <Typography
                sx={{
                  color: p.secondary,
                  fontSize: 16.5,
                  maxWidth: 680,
                  lineHeight: 1.55,
                }}
              >
                {section.subhead}
              </Typography>
            </Stack>
            <Box
              sx={{
                display: "grid",
                gap: 1.6,
                gridTemplateColumns: {
                  xs: "1fr",
                  md: "repeat(2, minmax(0, 1fr))",
                  lg: "repeat(3, minmax(0, 1fr))",
                },
              }}
            >
              {section.items.map((item) => (
                <Stack
                  key={item.title}
                  spacing={1.1}
                  sx={{
                    border: `1px solid ${p.border}`,
                    borderRadius: `${tokens.radius.sm}px`,
                    bgcolor: p.surface,
                    boxShadow: p.light
                      ? "0 22px 70px rgba(103,65,180,0.08)"
                      : "0 18px 60px rgba(0,0,0,0.20)",
                    minHeight: 174,
                    p: 2.1,
                  }}
                >
                  <Stack direction="row" alignItems="center" spacing={1.2}>
                    <Box
                      sx={{
                        border: `1px solid ${p.border}`,
                        borderRadius: 1.5,
                        bgcolor: p.surfaceStrong,
                        color: tokens.color.accent.violet,
                        display: "grid",
                        height: 46,
                        placeItems: "center",
                        width: 46,
                        "& svg": { fontSize: 24 },
                      }}
                    >
                      {item.icon}
                    </Box>
                    {item.meta ? (
                      <Box
                        sx={{
                          ml: "auto",
                          color: tokens.color.accent.violet,
                          fontWeight: 950,
                          fontSize: 14,
                        }}
                      >
                        {item.meta}
                      </Box>
                    ) : null}
                  </Stack>
                  <Typography sx={{ fontSize: 18, fontWeight: 950 }}>
                    {item.title}
                  </Typography>
                  <Typography
                    sx={{
                      color: p.secondary,
                      fontSize: 14.5,
                      lineHeight: 1.55,
                    }}
                  >
                    {item.body}
                  </Typography>
                </Stack>
              ))}
            </Box>
          </Box>
        ))}

        {data.quote ? (
          <Stack
            direction={{ xs: "column", md: "row" }}
            alignItems={{ xs: "flex-start", md: "center" }}
            sx={{
              border: `1px solid ${p.strong}`,
              borderRadius: `${tokens.radius.sm}px`,
              bgcolor: p.surface,
              p: { xs: 2.4, md: 3.2 },
              gap: 2,
            }}
          >
            <BoltRounded
              sx={{ color: tokens.color.accent.violet, fontSize: 34 }}
            />
            <Typography
              sx={{
                flex: 1,
                fontSize: { xs: 24, md: 32 },
                fontWeight: 950,
                lineHeight: 1.12,
              }}
            >
              {data.quote}
            </Typography>
            <Button
              component={Link}
              href={withTheme("/studio")}
              variant="contained"
              endIcon={<ArrowForwardRounded />}
            >
              Start building
            </Button>
          </Stack>
        ) : null}
      </Stack>
    </Box>
  );
}
