// app/mobile/page.tsx — public marketing route.
//
// Ship real mobile apps end-to-end. One track per
// StackDecision.Mobile.Kind value, with the GateMobileBuild behavior
// and ledger entries the orchestrator actually writes.

import type { Metadata } from "next";
import Link from "next/link";
import { ArrowForwardRounded, CheckCircleRounded, CancelRounded } from "@mui/icons-material";
import { Box, Button, Stack, Typography } from "@mui/material";
import { tokens } from "../../../../packages/design-tokens";
import { MarketingHero } from "../../src/components/marketing/MarketingHero";
import { MarketingSection } from "../../src/components/marketing/MarketingSection";
import { getRequestContent } from "../../src/lib/i18n/request";

export const metadata: Metadata = {
  title: "Mobile — Ironflyer",
  description:
    "Expo, Android native, iOS native, Flutter — gated all the way. Real EAS, gradlew, and xcodebuild dispatch with per-minute ledger entries.",
  alternates: { canonical: "https://ironflyer.com/mobile" },
  openGraph: {
    title: "Mobile — Ironflyer",
    description:
      "Expo, Android native, iOS native, Flutter — gated all the way. Real EAS, gradlew, and xcodebuild dispatch with per-minute ledger entries.",
    url: "https://ironflyer.com/mobile",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Mobile — Ironflyer",
    description:
      "Expo, Android native, iOS native, Flutter — gated all the way. Real EAS, gradlew, and xcodebuild dispatch with per-minute ledger entries.",
  },
};

interface Track {
  kind: string;
  label: string;
  tier: "Free" | "Pro";
  mac: string;
  signing: string;
  gate: string;
  ledger: string[];
  description: string;
}

const TRACKS: Track[] = [
  {
    kind: "expo",
    label: "Expo + EAS Build",
    tier: "Free",
    mac: "Not required",
    signing: "EAS-managed iOS + Android certs",
    gate: "Validates app.json, bundle ID, secrets, then dispatches `eas build`.",
    ledger: ["EntryEASBuildCredit", "EntryAppetizeMin"],
    description: "Recommended path. Ships real native iOS and Android binaries without a Mac in our pool.",
  },
  {
    kind: "react-native-bare",
    label: "React Native (bare)",
    tier: "Free",
    mac: "Required for iOS",
    signing: "Project-owned keystore + p12",
    gate: "Validates native folders on disk, runs `gradlew assembleDebug`, defers iOS to Mac pool.",
    ledger: ["EntryMobileBuildMin", "EntryEmulatorMin", "EntryMacWorkspaceMin"],
    description: "For teams that ejected from Expo and own their native folders. Android builds Linux-only.",
  },
  {
    kind: "android-native",
    label: "Android native (Kotlin)",
    tier: "Free",
    mac: "Not required",
    signing: "Project-owned keystore",
    gate: "Validates build.gradle, bundle ID via domain.AppIDPattern, runs `gradlew assembleDebug`.",
    ledger: ["EntryMobileBuildMin", "EntryEmulatorMin"],
    description: "Kotlin + Jetpack Compose starters. Linux sandbox handles the full lifecycle including emulator.",
  },
  {
    kind: "ios-native",
    label: "iOS native (Swift)",
    tier: "Pro",
    mac: "Required (mac pool)",
    signing: "Project-owned p12 + provisioning profile",
    gate: "Validates xcodegen.yml + xcodeproj, secrets, dispatches `xcodebuild build` on the mac pool.",
    ledger: ["EntryMacWorkspaceMin", "EntryMobileBuildMin"],
    description: "Swift + SwiftUI. Requires Pro tier. ProfitGuard refuses Mac allocations that push the wallet negative.",
  },
  {
    kind: "flutter",
    label: "Flutter",
    tier: "Free",
    mac: "Required for iOS",
    signing: "Project-owned keystore + p12",
    gate: "Validates pubspec.yaml, runs `flutter build apk` on Linux; iOS deferred to Mac pool or EAS-style fallback.",
    ledger: ["EntryMobileBuildMin", "EntryEmulatorMin", "EntryMacWorkspaceMin"],
    description: "Dart + Flutter. Android builds on Linux without a Mac; iOS needs the pool.",
  },
];

const FLOW = [
  { step: "Manifest validation", body: "Expo app.json / Android build.gradle / iOS xcodegen.yml / Flutter pubspec.yaml parsed." },
  { step: "Bundle ID check", body: "Reverse-DNS pattern enforced via domain.AppIDPattern. No co.test.placeholder slipping to the store." },
  { step: "Secrets verify", body: "Signing material confirmed present in Project.Secrets; never serialised to the client." },
  { step: "Build dispatch", body: "gradlew assembleDebug, xcodebuild build, or eas build runs on a real workspace." },
  { step: "Artifact verification", body: "APK / IPA / AAB lands at the expected path and is recorded in the ledger." },
];

const TIER_COLOR: Record<Track["tier"], string> = {
  Free: tokens.color.brand.mint,
  Pro: tokens.color.accent.violet,
};

export default async function MobilePage() {
  const { pages } = await getRequestContent();
  const hero = pages.mobile;

  return (
    <Box>
      <MarketingHero
        eyebrow={hero.eyebrow}
        title={hero.title}
        accentText={hero.titleAccent}
        subhead={hero.subhead}
        primary={{ href: "/signup", label: hero.primary }}
        secondary={{ href: "/solutions", label: hero.secondary }}
        proofChips={hero.proofChips}
      />

      <MarketingSection
        eyebrow="five tracks"
        title="One track per StackDecision.Mobile.Kind."
        subhead="Each track names its tier, Mac pool requirement, signing material, gate behavior, and the ledger entries the orchestrator writes per minute."
      >
        <Stack spacing={2.4}>
          {TRACKS.map((t) => (
            <Box
              key={t.kind}
              sx={{
                display: "grid",
                gridTemplateColumns: { xs: "1fr", md: "220px 1fr" },
                gap: { xs: 2, md: 3 },
                p: { xs: 2.4, md: 3 },
                borderRadius: 2,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.surface,
              }}
            >
              <Stack spacing={1}>
                <Typography
                  sx={{
                    fontFamily: tokens.font.mono,
                    fontSize: 12,
                    color: tokens.color.text.muted,
                    letterSpacing: 0.6,
                  }}
                >
                  {t.kind}
                </Typography>
                <Typography
                  sx={{
                    fontSize: 19,
                    fontWeight: 800,
                    color: tokens.color.text.primary,
                    letterSpacing: -0.2,
                  }}
                >
                  {t.label}
                </Typography>
                <Box
                  sx={{
                    alignSelf: "flex-start",
                    px: 1.1,
                    py: 0.4,
                    borderRadius: 999,
                    border: `1px solid ${TIER_COLOR[t.tier]}66`,
                    bgcolor: `${TIER_COLOR[t.tier]}1a`,
                  }}
                >
                  <Typography
                    sx={{
                      fontFamily: tokens.font.mono,
                      fontSize: 11,
                      letterSpacing: 1.2,
                      textTransform: "uppercase",
                      fontWeight: 700,
                      color: TIER_COLOR[t.tier],
                    }}
                  >
                    {t.tier} tier
                  </Typography>
                </Box>
              </Stack>
              <Stack spacing={1.4}>
                <Typography
                  sx={{ fontSize: 15, lineHeight: 1.6, color: tokens.color.text.secondary }}
                >
                  {t.description}
                </Typography>
                <Box
                  sx={{
                    display: "grid",
                    gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))" },
                    gap: 1.2,
                  }}
                >
                  {[
                    ["Mac pool", t.mac],
                    ["Signing", t.signing],
                  ].map(([label, value]) => (
                    <Box
                      key={label}
                      sx={{
                        p: 1.4,
                        borderRadius: 1.5,
                        bgcolor: tokens.color.bg.inset,
                        border: `1px solid ${tokens.color.border.subtle}`,
                      }}
                    >
                      <Typography
                        sx={{
                          fontFamily: tokens.font.mono,
                          fontSize: 10.5,
                          letterSpacing: 1.2,
                          textTransform: "uppercase",
                          color: tokens.color.text.muted,
                        }}
                      >
                        {label}
                      </Typography>
                      <Typography
                        sx={{ fontSize: 13.5, color: tokens.color.text.primary, mt: 0.3 }}
                      >
                        {value}
                      </Typography>
                    </Box>
                  ))}
                </Box>
                <Box
                  sx={{
                    p: 1.6,
                    borderRadius: 1.5,
                    bgcolor: `${tokens.color.accent.violet}10`,
                    border: `1px solid ${tokens.color.accent.violet}44`,
                  }}
                >
                  <Typography
                    sx={{
                      fontFamily: tokens.font.mono,
                      fontSize: 10.5,
                      letterSpacing: 1.2,
                      textTransform: "uppercase",
                      color: tokens.color.accent.violet,
                      fontWeight: 700,
                    }}
                  >
                    GateMobileBuild
                  </Typography>
                  <Typography
                    sx={{ fontSize: 13.5, color: tokens.color.text.secondary, mt: 0.4 }}
                  >
                    {t.gate}
                  </Typography>
                </Box>
                <Stack direction="row" spacing={0.8} flexWrap="wrap" useFlexGap>
                  {t.ledger.map((l) => (
                    <Box
                      key={l}
                      sx={{
                        px: 1,
                        py: 0.4,
                        borderRadius: 999,
                        border: `1px solid ${tokens.color.border.subtle}`,
                        bgcolor: tokens.color.bg.inset,
                        fontFamily: tokens.font.mono,
                        fontSize: 11,
                        color: tokens.color.brand.mint,
                      }}
                    >
                      {l}
                    </Box>
                  ))}
                </Stack>
              </Stack>
            </Box>
          ))}
        </Stack>
      </MarketingSection>

      <MarketingSection
        bgVariant="inset"
        eyebrow="execution flow"
        title="How a mobile build moves through the gate."
        subhead="Five stages, each named, each blocking. A mobile run that says 'building' without telling you which stage is open is a regression."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr",
              md: "repeat(5, minmax(0, 1fr))",
            },
            gap: 2,
          }}
        >
          {FLOW.map((s, idx) => (
            <Box
              key={s.step}
              sx={{
                position: "relative",
                p: 2.2,
                borderRadius: 2,
                border: `1px solid ${tokens.color.border.subtle}`,
                bgcolor: tokens.color.bg.surface,
              }}
            >
              <Typography
                sx={{
                  fontFamily: tokens.font.mono,
                  fontSize: 11,
                  letterSpacing: 1.4,
                  textTransform: "uppercase",
                  color: tokens.color.accent.coral,
                  fontWeight: 700,
                }}
              >
                Step {idx + 1}
              </Typography>
              <Typography
                sx={{
                  mt: 0.6,
                  fontSize: 16,
                  fontWeight: 800,
                  color: tokens.color.text.primary,
                  letterSpacing: -0.2,
                }}
              >
                {s.step}
              </Typography>
              <Typography
                sx={{ mt: 0.8, fontSize: 13.5, lineHeight: 1.55, color: tokens.color.text.secondary }}
              >
                {s.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </MarketingSection>

      <MarketingSection
        eyebrow="pro tier"
        title="Mac pool callout."
        subhead="Apple-licensed hardware is expensive. We make the cost visible per minute and let ProfitGuard refuse allocations the wallet cannot absorb."
      >
        <Box
          sx={{
            p: { xs: 3, md: 4 },
            borderRadius: 3,
            border: `1px solid ${tokens.color.border.strong}`,
            bgcolor: tokens.color.bg.surface,
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: 3,
          }}
        >
          <Stack spacing={1.5}>
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 11.5,
                letterSpacing: 1.4,
                textTransform: "uppercase",
                color: tokens.color.accent.violet,
                fontWeight: 700,
              }}
            >
              Real numbers
            </Typography>
            <Typography
              sx={{
                fontSize: { xs: 22, md: 28 },
                fontWeight: 900,
                letterSpacing: -0.4,
                color: tokens.color.text.primary,
                lineHeight: 1.2,
              }}
            >
              $130–500 / month per concurrent Mac workspace.
            </Typography>
            <Typography sx={{ fontSize: 15, lineHeight: 1.6, color: tokens.color.text.secondary }}>
              Scaleway, MacStadium, AWS mac2.metal — they all land in that band.
              We surface the per-minute rate live on the wallet panel and refuse
              allocations that would push your wallet negative.
            </Typography>
          </Stack>
          <Stack spacing={1.5}>
            <Typography
              sx={{
                fontFamily: tokens.font.mono,
                fontSize: 11.5,
                letterSpacing: 1.4,
                textTransform: "uppercase",
                color: tokens.color.brand.mint,
                fontWeight: 700,
              }}
            >
              ProfitGuard mechanic
            </Typography>
            <Typography sx={{ fontSize: 15, lineHeight: 1.6, color: tokens.color.text.secondary }}>
              Reservation runs before any xcodebuild call. If the projected cost
              exceeds available wallet, the orchestrator returns HTTP 402 with a
              top_up_url and the gate verdict names the open mechanic.
            </Typography>
            <Box
              sx={{
                p: 1.4,
                borderRadius: 1.5,
                bgcolor: tokens.color.bg.inset,
                border: `1px solid ${tokens.color.border.subtle}`,
                fontFamily: tokens.font.mono,
                fontSize: 12.5,
                color: tokens.color.text.primary,
                whiteSpace: "pre-wrap",
              }}
            >
              {`profitguard.reserve(macHours)\n  → wallet.available < projected\n  → 402 Payment Required`}
            </Box>
          </Stack>
        </Box>
      </MarketingSection>

      <MarketingSection
        bgVariant="inset"
        eyebrow="comparison"
        title="Bolt, Lovable, v0 don't ship native mobile builds. We do."
      >
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "repeat(4, minmax(0, 1fr))" },
            gap: 1.6,
          }}
        >
          {[
            { name: "Ironflyer", value: "Expo + Android + iOS + Flutter, gated", ok: true },
            { name: "Bolt", value: "Web only", ok: false },
            { name: "Lovable", value: "Web only", ok: false },
            { name: "v0", value: "Web components only", ok: false },
          ].map((row) => (
            <Box
              key={row.name}
              sx={{
                p: 2.2,
                borderRadius: 2,
                border: `1px solid ${row.ok ? tokens.color.border.strong : tokens.color.border.subtle}`,
                bgcolor: row.ok ? `${tokens.color.brand.mint}10` : tokens.color.bg.surface,
              }}
            >
              <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 0.6 }}>
                {row.ok ? (
                  <CheckCircleRounded sx={{ fontSize: 18, color: tokens.color.brand.mint }} />
                ) : (
                  <CancelRounded sx={{ fontSize: 18, color: tokens.color.text.muted }} />
                )}
                <Typography sx={{ fontWeight: 800, color: tokens.color.text.primary }}>
                  {row.name}
                </Typography>
              </Stack>
              <Typography sx={{ fontSize: 13.5, color: tokens.color.text.secondary }}>
                {row.value}
              </Typography>
            </Box>
          ))}
        </Box>

        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={1.5}
          sx={{ pt: 4, justifyContent: "center" }}
        >
          <Button
            component={Link}
            href="/signup"
            variant="contained"
            color="primary"
            size="large"
            endIcon={<ArrowForwardRounded sx={{ fontSize: 18 }} />}
          >
            Start a mobile build
          </Button>
          <Button
            component={Link}
            href="/pricing"
            variant="text"
            size="large"
            sx={{ color: tokens.color.accent.violet }}
          >
            Review pricing
          </Button>
        </Stack>
      </MarketingSection>
    </Box>
  );
}
