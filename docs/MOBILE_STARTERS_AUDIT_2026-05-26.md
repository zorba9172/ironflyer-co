# Mobile Starters Audit — 2026-05-26

Health audit of the three mobile starters under
`templates/starters/{react-native-expo,android-kotlin,ios-swift}`. Trivial
production-readiness fixes were applied in the same pass. Version bumps
(Expo SDK, Kotlin/AGP, target SDK) are intentionally NOT performed here —
those are operator decisions.

No tests were touched, written, or run (per the repo's constitutional
no-tests rule). Native build / lint commands were NOT executed — too heavy
for this audit; obvious syntax / config issues were flagged by inspection.

---

## 1. `react-native-expo`

| Surface | Status |
| --- | --- |
| Expo SDK | `expo: ~53.0.0` (current stable: SDK 53; Expo 54 in canary). **Up-to-date.** |
| React Native | `0.76.5` (matched to Expo 53 New Arch baseline). |
| React | `18.3.1`. |
| `expo-router` | `~4.0.0`. |
| `app.json` | `name`, `slug`, `version`, `orientation`, `userInterfaceStyle`, `scheme`, `newArchEnabled`, iOS/Android bundle ids, splash, plugins — all present. |
| `app.config.ts` | Typed extension; injects `EAS_PROJECT_ID` from env. Clean. |
| `eas.json` | 3 profiles (development / preview / production) — clean. |
| `.env.example` | `EXPO_PUBLIC_API_BASE=https://api.example.com` — placeholder, env-driven (no hardcoded URLs in source). |
| `console.log` | None. |
| Hardcoded URLs in source | None (only the placeholder in `.env.example`). |
| README | Matches the current Ironflyer flow (EAS profiles, bundle-id gate, ProfitGuard metering). |
| Pinning style | `~` (caret-minor) for Expo packages; specific version for React + RN. Matches Expo's recommended pin discipline. |

**Production-readiness issues:** none material. The `theme` const at the
top of `app/index.tsx` and `app/dashboard.tsx` uses raw hex literals — this
is INTENTIONAL per the starter's README ("Colors are defined inline at the
top of each screen as a `theme` const so the starter does not depend on a
design-tokens package that does not exist in user workspaces"). The
constitutional design-tokens rule applies to `clients/web`, not to user
workspace scaffolding.

**Trivial fixes applied:** none needed.

---

## 2. `android-kotlin`

| Surface | Status |
| --- | --- |
| AGP | `com.android.application 8.7.2` (current stable; AGP 8.8 is canary). |
| Kotlin | `2.0.21` (current stable; 2.1.x is bleeding edge). |
| Kotlin Compose plugin | `2.0.21` (matched). |
| `compileSdk` / `targetSdk` | 35 (current Android Play Store baseline for Aug 2025 deadline). |
| `minSdk` | 24 (Android 7.0; reasonable floor). |
| JDK | 17 (current AGP requirement). |
| Compose BOM | `2024.12.01` (current). |
| Navigation Compose | `2.8.4` (current). |
| `gradle.properties` | `useAndroidX`, `nonTransitiveRClass`, `parallel`, `caching`, `kotlin.code.style=official` — clean. |
| `AndroidManifest.xml` | `INTERNET` permission only; `allowBackup="false"`; `dataExtractionRules` + `backupRules` wired; single `MainActivity` with `LAUNCHER` intent filter. **No `android:debuggable="true"`.** |
| Application class | None declared in manifest (relies on default Application). Acceptable for a starter. |
| `release` build type | `isMinifyEnabled = true` + ProGuard wiring. Clean. |
| `console.log` / `Log.d` left over | None. |
| README | Matches current Ironflyer flow (gate-enforced `applicationId`, signing-secret validation, ProfitGuard emulator metering, GitHub Actions workflows, fastlane lanes). |

**Production-readiness issues:** none material. The `release` build type
declares no `signingConfig` — that is intentional (signing is provided by
the GitHub Actions workflow + the `ANDROID_KEYSTORE_*` secrets enumerated
in the README).

**Trivial fixes applied:** none needed.

---

## 3. `ios-swift`

| Surface | Status |
| --- | --- |
| Project model | xcodegen-driven; no binary `.xcodeproj` in tree (correct — README documents this). |
| `Package.swift` | swift-tools 5.10; LSP/editor support only (correct). |
| `xcodegen.yml` | deploymentTarget iOS 15.1; bundleIdPrefix `com.ironflyer`; CODE_SIGN_STYLE Automatic; resources wired. |
| Swift version | 5.10. |
| `Info.plist` | `CFBundleDevelopmentRegion=en`, `CFBundleDisplayName`, `CFBundleIdentifier` (templated), `CFBundleShortVersionString=0.1.0`, `CFBundleVersion=1`, `LSRequiresIPhoneOS=true`, `UILaunchScreen={}`, `UIRequiredDeviceCapabilities=[arm64]`, `UISupportedInterfaceOrientations`, iPad orientations. |
| `ITSAppUsesNonExemptEncryption` | **Missing.** Apple now requires the key (or an export-compliance docs upload) on App Store submission. Adding this is a content decision (true vs false) for the operator — flagged as a follow-up rather than an auto-fix. |
| `PrivacyInfo.xcprivacy` | **Was missing.** Apple's privacy manifest is mandatory in 2024+. **Added in this pass** at `Resources/PrivacyInfo.xcprivacy`, wired into `xcodegen.yml` resources. Default content: tracking=false, no collected data types, declares `NSPrivacyAccessedAPICategoryUserDefaults` with reason code `CA92.1` (App-functionality default; safe baseline). Operators add additional API categories as their app evolves (FileTimestamp `C617.1`, SystemBootTime `35F9.1`, DiskSpace `E174.1`, etc.). |
| `App.swift` / `ContentView.swift` / `DashboardView.swift` / `Theme.swift` | Clean SwiftUI; no `print()` left over; navigation via `NavigationStack` + `NavigationLink(value:)`. |
| Makefile | `make generate` / `make build` / `make clean` — clean. |
| fastlane | `Appfile` + `Fastfile` + `Matchfile` present (separate from the audit; documented in README). |
| README | Matches current Ironflyer flow (bundle-id gate, signing validation, Mac-pool metering, GitHub Actions, fastlane). |

**Production-readiness issues:**
1. (Fixed) `PrivacyInfo.xcprivacy` was absent; App Store rejects submissions
   without one since 2024-Q1. Added with conservative defaults.
2. (Flagged) `ITSAppUsesNonExemptEncryption` is not in `Info.plist`. The
   operator must set this to `<false/>` for apps that only use Apple's
   stock TLS / HTTPS APIs, or document export-compliance via App Store
   Connect. Leaving the decision to the operator rather than presuming.

**Trivial fixes applied:**
- Added `Resources/PrivacyInfo.xcprivacy` (privacy manifest, minimum
  acceptable for App Store submission).
- Wired the new resource into `xcodegen.yml` under `targets.IronflyerStarter.resources`.

---

## Summary

| Starter | Versions current? | Hardcoded URLs? | Debug flags? | Missing manifests? | Trivial fix applied |
| --- | --- | --- | --- | --- | --- |
| `react-native-expo` | yes (Expo SDK 53) | no | no | no | none needed |
| `android-kotlin` | yes (AGP 8.7.2, Kotlin 2.0.21, SDK 35) | no | no | no | none needed |
| `ios-swift` | yes (Swift 5.10, iOS 15.1+) | no | no | `PrivacyInfo.xcprivacy` was missing | **Added `Resources/PrivacyInfo.xcprivacy` + wired into `xcodegen.yml`** |

## Compile / lint posture

No native builds were executed (intentional — too heavy for a content
audit). Static inspection found no syntax issues. The three starters are
in a state where:
- `expo prebuild && expo run:android` should produce a debug APK on a
  workstation with Android SDK 35 + JDK 17.
- `./gradlew :app:assembleDebug` should produce a debug APK from the
  `android-kotlin` starter directly.
- `make generate && xcodebuild ... build` should produce a simulator
  build from the `ios-swift` starter on a Mac with Xcode 15+.

## Deferred (intentional)

- Expo SDK 53 → 54 bump when 54 ships stable.
- Kotlin 2.0.21 → 2.1.x bump when AGP catches up.
- `targetSdk` 35 → 36 when Android's Play Store deadline moves.
- iOS deployment target 15.1 → 16.0 when the operator's user base
  allows it.
- `ITSAppUsesNonExemptEncryption` decision (`<false/>` for stock TLS;
  `<true/>` requires App Store Connect export compliance docs).
