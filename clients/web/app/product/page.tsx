// app/product/page.tsx — public marketing surface explaining the
// finisher engine, gates, and Docker workspaces. Server component;
// nothing here owns client state.

import {
  AccountBalanceWalletRounded,
  AssignmentTurnedInRounded,
  AutoFixHighRounded,
  BoltRounded,
  BuildCircleRounded,
  CheckCircleRounded,
  CloudDoneRounded,
  CloudOffRounded,
  DesignServicesRounded,
  DnsRounded,
  FactCheckRounded,
  GavelRounded,
  HubRounded,
  PhoneIphoneRounded,
  PolicyRounded,
  PsychologyRounded,
  RocketLaunchRounded,
  RuleRounded,
  SecurityRounded,
  ShieldRounded,
  TerminalRounded,
  TypeSpecimenRounded,
  VerifiedUserRounded,
  VisibilityRounded,
} from "@mui/icons-material";
import { Box, Stack, Typography } from "@mui/material";
import type { Metadata } from "next";
import type { ReactNode } from "react";
import { tokens } from "../../../../packages/design-tokens";
import {
  CtaBand,
  MarketingHero,
  MarketingSection,
  MechanicCard,
} from "../../src/components/marketing";
import { getRequestContent } from "../../src/lib/i18n/request";

export const metadata: Metadata = {
  title: "Product — Ironflyer",
  description:
    "Ironflyer is the production discipline AI app builders skip. Gates that block, patches that can be reviewed, wallet enforced upfront, real Docker workspaces.",
  openGraph: {
    title: "Product — Ironflyer",
    description:
      "Gates, reviewable patches, prepaid wallet, and real Docker workspaces. The finisher engine that turns prompts into shipped product.",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Product — Ironflyer",
    description:
      "Gates, reviewable patches, prepaid wallet, real Docker workspaces — the finisher engine.",
  },
};

interface FlowStep {
  step: string;
  title: string;
  mechanic: string;
  body: string;
  icon: ReactNode;
}

const FLOW: FlowStep[] = [
  {
    step: "01",
    title: "Idea",
    mechanic: "intent capture",
    body: "Plain-language product brief lands in the orchestrator as an Execution with a typed goal.",
    icon: <PsychologyRounded />,
  },
  {
    step: "02",
    title: "Describe",
    mechanic: "blueprint",
    body: "AI Architect resolves the brief against blueprints + memory; stack, data, and roles are pinned before code.",
    icon: <DesignServicesRounded />,
  },
  {
    step: "03",
    title: "Plan",
    mechanic: "GateBudget",
    body: "Wallet hold is reserved; the plan is refused with 402 if balance is short. No reasoning runs unfunded.",
    icon: <AccountBalanceWalletRounded />,
  },
  {
    step: "04",
    title: "Patch loop",
    mechanic: "patch.Engine.Propose",
    body: "The agent never writes files directly. Every change is a patch that passes through review and lint gates.",
    icon: <AutoFixHighRounded />,
  },
  {
    step: "05",
    title: "Gates",
    mechanic: "finisher.DefaultGates",
    body: "Lint, typecheck, security scan, build, ProfitGuard, mobile build, deploy, E2E — all must clear.",
    icon: <RuleRounded />,
  },
  {
    step: "06",
    title: "Ship",
    mechanic: "OutcomeEvent",
    body: "Deploy artifact lands, ledger reconciles, OutcomeEvent emits to the feedback brain so the system learns.",
    icon: <RocketLaunchRounded />,
  },
];

interface Gate {
  name: string;
  role: string;
  icon: ReactNode;
}

const GATES: Gate[] = [
  {
    name: "GateBudget",
    role: "Wallet hold reserved or 402 Payment Required.",
    icon: <AccountBalanceWalletRounded />,
  },
  {
    name: "GatePatchReview",
    role: "Every AI-authored patch must pass review before apply.",
    icon: <FactCheckRounded />,
  },
  {
    name: "GateBuild",
    role: "Project compiles inside the sandbox before deploy is even considered.",
    icon: <BuildCircleRounded />,
  },
  {
    name: "GateMobileBuild",
    role: "Validates Expo / Gradle / Xcode manifest, then drives the real build.",
    icon: <PhoneIphoneRounded />,
  },
  {
    name: "GateLint",
    role: "Style + AST issues blocked before they enter main.",
    icon: <RuleRounded />,
  },
  {
    name: "GateTypecheck",
    role: "Type errors stop the patch; AI doesn't get to merge red TS.",
    icon: <TypeSpecimenRounded />,
  },
  {
    name: "GateSecurityScan",
    role: "Secrets, suspicious imports, and unsafe filesystem access are blocked.",
    icon: <SecurityRounded />,
  },
  {
    name: "GateProfitGuard",
    role: "Refuses any expensive call that would push expected margin negative.",
    icon: <ShieldRounded />,
  },
  {
    name: "GateDeploy",
    role: "Only signed-off, gate-green patches reach a deploy artifact.",
    icon: <CloudDoneRounded />,
  },
  {
    name: "GateE2E",
    role: "End-to-end run against the preview before a release is marked ready.",
    icon: <CheckCircleRounded />,
  },
  {
    name: "GateLicense",
    role: "Open-source licenses checked against allow/deny list per project.",
    icon: <GavelRounded />,
  },
  {
    name: "GateOwnerCheck",
    role: "Per-user isolation: refuses access to projects the caller does not own.",
    icon: <PolicyRounded />,
  },
];

export default async function ProductPage() {
  const { pages } = await getRequestContent();
  const hero = pages.product;

  return (
    <Box sx={{ width: "100%", minWidth: 0 }}>
      <MarketingHero
        eyebrow={hero.eyebrow}
        title={hero.title}
        accentText={hero.titleAccent}
        subhead={hero.subhead}
        primary={{ href: "/signup", label: hero.primary }}
        secondary={{ href: "/pricing", label: hero.secondary }}
        proofChips={hero.proofChips}
      />

      <MarketingSection
        eyebrow="how a paid execution flows"
        title="Six steps. Every one is a named mechanic."
        subhead="From plain-language brief to deploy artifact, the orchestrator hands the work to a typed pipeline. Each step writes to the ledger; each transition is observable."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, 1fr)",
              lg: "repeat(6, 1fr)",
            },
            gap: { xs: 2, md: 1.6 },
            position: "relative",
          }}
        >
          {FLOW.map((step) => (
            <Box
              key={step.step}
              sx={{
                p: 2.2,
                borderRadius: `${tokens.radius.md}px`,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.surfaceRaised}d9`,
                display: "flex",
                flexDirection: "column",
                gap: 1.2,
                minHeight: 200,
              }}
            >
              <Stack direction="row" alignItems="center" spacing={1}>
                <Box
                  sx={{
                    display: "inline-grid",
                    placeItems: "center",
                    width: 30,
                    height: 30,
                    borderRadius: `${tokens.radius.sm}px`,
                    bgcolor: `${tokens.color.accent.violet}26`,
                    color: tokens.color.accent.violet,
                    "& svg": { fontSize: 18 },
                  }}
                >
                  {step.icon}
                </Box>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 11,
                    color: tokens.color.accent.violet,
                    fontWeight: 700,
                    letterSpacing: 1,
                  }}
                >
                  {step.step}
                </Typography>
              </Stack>
              <Typography
                sx={{
                  fontSize: 17,
                  fontWeight: 800,
                  color: tokens.color.text.primary,
                  letterSpacing: -0.2,
                }}
              >
                {step.title}
              </Typography>
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11.5,
                  color: tokens.color.accent.violet,
                  letterSpacing: 0.3,
                }}
              >
                {step.mechanic}
              </Typography>
              <Typography
                sx={{
                  color: tokens.color.text.secondary,
                  fontSize: 12.5,
                  lineHeight: 1.55,
                  flex: 1,
                }}
              >
                {step.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="gates catalog"
        title="Twelve gates, registered with the finisher engine."
        subhead="Every gate takes (ctx, *GateEnv) and returns a typed verdict. If a verdict blocks, the orchestrator pauses the execution and surfaces the reason — never a silent retry."
        bgVariant="inset"
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, 1fr)",
              md: "repeat(3, 1fr)",
              lg: "repeat(4, 1fr)",
            },
            gap: 2,
          }}
        >
          {GATES.map((gate) => (
            <MechanicCard
              key={gate.name}
              name={gate.name}
              description={gate.role}
              icon={gate.icon}
            />
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="the finisher engine"
        title="Gate registry → patch engine → ledger → OutcomeEvent."
        subhead="Every paid execution traces the same chain. The finisher is the thing that makes 'shipped' an objective verdict, not a vibe."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: { xs: 3, md: 4 },
            alignItems: "stretch",
          }}
        >
          <Stack spacing={2.2}>
            {[
              {
                icon: <HubRounded />,
                title: "Gate registry",
                body: "DefaultGates() returns the ordered chain. Custom gates plug in via Register(name, fn).",
              },
              {
                icon: <AutoFixHighRounded />,
                title: "Patch engine",
                body: "patch.Engine.Propose is the only write path. Every diff carries author, scope, and gate trace.",
              },
              {
                icon: <DnsRounded />,
                title: "Ledger entry",
                body: "Reservation, debit, refund — every transition lands in the append-only ledger.",
              },
              {
                icon: <AssignmentTurnedInRounded />,
                title: "OutcomeEvent",
                body: "learning.Publish emits the typed outcome; the pattern miner uses it to evolve strategy.",
              },
            ].map((item) => (
              <Stack
                key={item.title}
                direction="row"
                alignItems="flex-start"
                spacing={2}
                sx={{
                  p: 2.4,
                  borderRadius: `${tokens.radius.md}px`,
                  border: `1px solid ${tokens.color.border.subtle}`,
                  bgcolor: `${tokens.color.bg.surface}cc`,
                }}
              >
                <Box
                  sx={{
                    display: "inline-grid",
                    placeItems: "center",
                    width: 38,
                    height: 38,
                    borderRadius: `${tokens.radius.sm}px`,
                    bgcolor: `${tokens.color.accent.violet}1f`,
                    color: tokens.color.accent.violet,
                    "& svg": { fontSize: 20 },
                    flexShrink: 0,
                  }}
                >
                  {item.icon}
                </Box>
                <Stack spacing={0.6} sx={{ minWidth: 0 }}>
                  <Typography
                    sx={{
                      fontSize: 16,
                      fontWeight: 800,
                      color: tokens.color.text.primary,
                    }}
                  >
                    {item.title}
                  </Typography>
                  <Typography
                    sx={{
                      color: tokens.color.text.secondary,
                      fontSize: 13.5,
                      lineHeight: 1.6,
                    }}
                  >
                    {item.body}
                  </Typography>
                </Stack>
              </Stack>
            ))}
          </Stack>

          <Box
            role="img"
            aria-label="execution.png — finisher engine pipeline screenshot"
            sx={{
              minHeight: 460,
              borderRadius: `${tokens.radius.lg}px`,
              border: `1px solid ${tokens.color.border.accent}`,
              bgcolor: tokens.color.bg.surfaceRaised,
              background: `linear-gradient(180deg, ${tokens.color.bg.surfaceRaised}f5, ${tokens.color.bg.inset}f5)`,
              position: "relative",
              overflow: "hidden",
              display: "flex",
              flexDirection: "column",
            }}
          >
            <Stack
              direction="row"
              spacing={1}
              alignItems="center"
              sx={{
                px: 2,
                py: 1.4,
                borderBottom: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: `${tokens.color.bg.inset}b3`,
              }}
            >
              {[
                tokens.color.accent.danger,
                tokens.color.accent.warning,
                tokens.color.accent.success,
              ].map((c, i) => (
                <Box
                  key={i}
                  sx={{
                    width: 10,
                    height: 10,
                    borderRadius: "50%",
                    bgcolor: c,
                  }}
                />
              ))}
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  color: tokens.color.text.muted,
                  ml: 1.5,
                }}
              >
                execution.png — exec_4a1f3c
              </Typography>
            </Stack>
            <Box sx={{ p: 2.2, flex: 1, display: "flex", flexDirection: "column", gap: 1.2 }}>
              {[
                { label: "GateBudget", status: "pass", note: "reserved $0.42" },
                { label: "GateLint", status: "pass", note: "0 findings" },
                { label: "GateTypecheck", status: "pass", note: "0 errors" },
                { label: "GateSecurityScan", status: "pass", note: "0 secrets" },
                { label: "GatePatchReview", status: "pass", note: "approved" },
                { label: "GateBuild", status: "pass", note: "next build · 18.2s" },
                { label: "GateProfitGuard", status: "pass", note: "margin +$0.31" },
                { label: "GateE2E", status: "running", note: "playwright · 4/9" },
                { label: "GateDeploy", status: "pending", note: "waiting on E2E" },
              ].map((row) => {
                const color =
                  row.status === "pass"
                    ? tokens.color.accent.success
                    : row.status === "running"
                      ? tokens.color.accent.violet
                      : tokens.color.text.muted;
                return (
                  <Stack
                    key={row.label}
                    direction="row"
                    alignItems="center"
                    spacing={1.5}
                    sx={{
                      px: 1.4,
                      py: 1,
                      borderRadius: `${tokens.radius.sm}px`,
                      border: `1px solid ${tokens.color.border.subtle}`,
                      bgcolor: `${tokens.color.bg.base}80`,
                    }}
                  >
                    <Box
                      sx={{
                        width: 8,
                        height: 8,
                        borderRadius: "50%",
                        bgcolor: color,
                        flexShrink: 0,
                      }}
                    />
                    <Typography
                      sx={{
                        fontFamily: tokens.font.mono,
                        fontSize: 12,
                        color: tokens.color.text.primary,
                        flex: 1,
                      }}
                    >
                      {row.label}
                    </Typography>
                    <Typography
                      sx={{
                        fontFamily: tokens.font.mono,
                        fontSize: 11,
                        color: tokens.color.text.muted,
                      }}
                    >
                      {row.note}
                    </Typography>
                    <Typography
                      sx={{
                        fontFamily: tokens.font.mono,
                        fontSize: 11,
                        color,
                        textTransform: "uppercase",
                        letterSpacing: 0.6,
                        minWidth: 64,
                        textAlign: "right",
                      }}
                    >
                      {row.status}
                    </Typography>
                  </Stack>
                );
              })}
            </Box>
          </Box>
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="real Docker workspaces, not browser sandboxes"
        title="The orchestrator runs your code in a real Linux box."
        subhead="Per-user sandboxes are provisioned by the runtime service. No webcontainer half-truths, no shared host. Mobile gets the same posture, on the right hardware."
        bgVariant="inset"
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: 3,
          }}
        >
          <Box
            sx={{
              p: { xs: 3, md: 3.5 },
              borderRadius: `${tokens.radius.lg}px`,
              border: `1px solid ${tokens.color.accent.violet}66`,
              background: `linear-gradient(180deg, ${tokens.color.bg.surfaceRaised}f2, ${tokens.color.bg.inset}f5)`,
              display: "flex",
              flexDirection: "column",
              gap: 2,
            }}
          >
            <Stack direction="row" alignItems="center" spacing={1.5}>
              <TerminalRounded
                sx={{
                  fontSize: 28,
                  color: tokens.color.accent.violet,
                }}
              />
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 13,
                  color: tokens.color.accent.violet,
                  fontWeight: 700,
                  letterSpacing: 0.8,
                }}
              >
                Ironflyer
              </Typography>
            </Stack>
            <Typography
              sx={{
                fontSize: { xs: 22, md: 26 },
                fontWeight: 800,
                color: tokens.color.text.primary,
                letterSpacing: -0.3,
              }}
            >
              Real Docker. Real KVM. Real Mac.
            </Typography>
            <Stack spacing={1.2}>
              {[
                { icon: <TerminalRounded />, label: "Mock driver for dev — fast and free" },
                { icon: <DnsRounded />, label: "Docker driver for prod sandboxes" },
                { icon: <PhoneIphoneRounded />, label: "KVM passthrough for Android emulator" },
                { icon: <BoltRounded />, label: "MacStadium pool for iOS native (Enterprise)" },
                { icon: <VerifiedUserRounded />, label: "Per-user OwnerID isolation on every workspace" },
              ].map((row) => (
                <Stack
                  key={row.label}
                  direction="row"
                  alignItems="center"
                  spacing={1.5}
                >
                  <Box
                    sx={{
                      color: tokens.color.accent.violet,
                      "& svg": { fontSize: 18 },
                    }}
                  >
                    {row.icon}
                  </Box>
                  <Typography
                    sx={{
                      fontSize: 13.5,
                      color: tokens.color.text.primary,
                      fontWeight: 600,
                    }}
                  >
                    {row.label}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          </Box>

          <Box
            sx={{
              p: { xs: 3, md: 3.5 },
              borderRadius: `${tokens.radius.lg}px`,
              border: `1px solid ${tokens.color.border.subtle}`,
              bgcolor: `${tokens.color.bg.surface}cc`,
              display: "flex",
              flexDirection: "column",
              gap: 2,
            }}
          >
            <Stack direction="row" alignItems="center" spacing={1.5}>
              <CloudOffRounded
                sx={{
                  fontSize: 28,
                  color: tokens.color.text.muted,
                }}
              />
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 13,
                  color: tokens.color.text.muted,
                  fontWeight: 700,
                  letterSpacing: 0.8,
                }}
              >
                Prompt-to-app crowd
              </Typography>
            </Stack>
            <Typography
              sx={{
                fontSize: { xs: 22, md: 26 },
                fontWeight: 800,
                color: tokens.color.text.muted,
                letterSpacing: -0.3,
              }}
            >
              Browser sandboxes. Demo only.
            </Typography>
            <Stack spacing={1.2}>
              {[
                "Webcontainer — no real Linux syscalls",
                "Shared tab process — no per-user isolation",
                "No Android emulator, no iOS native path",
                "No persistent workspace state",
                "No Docker, no KVM, no Mac pool",
              ].map((row) => (
                <Stack key={row} direction="row" alignItems="center" spacing={1.5}>
                  <Box
                    sx={{
                      width: 8,
                      height: 8,
                      borderRadius: "50%",
                      bgcolor: tokens.color.text.muted,
                    }}
                  />
                  <Typography
                    sx={{
                      fontSize: 13.5,
                      color: tokens.color.text.muted,
                    }}
                  >
                    {row}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          </Box>
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="visualization-first"
        title="The AI's state is the product. We show it."
        subhead="Every cockpit, studio, execution, profit, and wallet surface lands on a visual mirror of orchestrator state first. VS Code is one click away, never the default."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
            gap: 2,
          }}
        >
          <MechanicCard
            name="executionFeed"
            description="GraphQL subscription mirroring every gate transition, patch verdict, and cost line in real time."
            icon={<VisibilityRounded />}
          />
          <MechanicCard
            name="ProfitDashboard"
            description="Margin first. Scale metrics only matter when margin is healthy. Live binding to the ledger."
            icon={<ShieldRounded />}
            accent="mint"
          />
          <MechanicCard
            name="Studio · what's unclosed"
            description="Names the pending gate, the missing artifact, the unresolved patch. Running is never a vibe."
            icon={<RuleRounded />}
            accent="coral"
          />
        </Box>
      </MarketingSection>

      <CtaBand
        heading="Stop shipping demos. Start shipping product."
        sub="Every Ironflyer execution carries a wallet hold, a patch trace, and a gate verdict. The orchestrator refuses the rest."
        primary={{ href: "/signup", label: "Open a workspace" }}
        secondary={{ href: "/pricing", label: "See the wallet model" }}
        chips={[
          "Free tier with mock Docker",
          "Pro from $29 + top-ups",
          "Enterprise for iOS native + SSO",
        ]}
      />
    </Box>
  );
}
