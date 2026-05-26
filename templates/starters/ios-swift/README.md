# iOS native starter (Ironflyer)

Swift + SwiftUI starter that Ironflyer's mobile build gate scaffolds into
your workspace. Text-only — the Xcode project is regenerated from
`xcodegen.yml` so there is no binary `.xcodeproj` to merge-conflict over.

## Prerequisites

- macOS host with Xcode 15 or newer
- [XcodeGen](https://github.com/yonaskolb/XcodeGen): `brew install xcodegen`

## Run it

```sh
make generate            # produces IronflyerStarter.xcodeproj
open IronflyerStarter.xcodeproj
```

Or build from the command line (what the gate does):

```sh
make build
```

## What's inside

- `Sources/IronflyerStarter/` — SwiftUI app (`App`, `ContentView`,
  `DashboardView`, `Theme`)
- `Resources/Assets.xcassets/` — accent color + placeholder app icon
- `Resources/Info.plist` — bundle metadata
- `xcodegen.yml` — project spec consumed by `xcodegen`
- `Package.swift` — SwiftPM manifest so SourceKit-LSP / VSCode Swift
  picks up the sources without xcodegen having run

## What Ironflyer adds

- **Gate-enforced bundle identifier.** Change `com.ironflyer.starter` in
  `xcodegen.yml` before your first release — the MobileBuildGate will
  refuse to ship the placeholder ID.
- **Signing certificates validated before release.** Provisioning
  profiles and `.p12` material are checked end-to-end so a broken
  certificate never leaks into a public build.
- **ProfitGuard meters Mac workspace minutes.** Building iOS requires a
  Mac runner; usage is metered against your iOS Pro tier so you always
  know the marginal cost of the next build.

## CI/CD

GitHub Actions workflows ship with this starter under `.github/workflows/`:

- [`ios-build.yml`](.github/workflows/ios-build.yml) — builds the
  Debug configuration for the iPhone 16 simulator on every push to
  `main` and on manual triggers. Runs on `macos-15` because xcodebuild
  only ships on Apple-licensed runners.
- [`ios-testflight.yml`](.github/workflows/ios-testflight.yml) —
  manual-only. Uses [`fastlane match`](https://docs.fastlane.tools/actions/match/)
  for signing material and uploads the Release archive to TestFlight via
  App Store Connect.

Fastlane lanes live under [`fastlane/`](fastlane/):

- `bundle exec fastlane simulator` — Debug build for the generic iOS
  Simulator destination.
- `bundle exec fastlane beta` — sync match, bump build number, archive
  the Release configuration, and upload to TestFlight.

### Required GitHub secrets

| Secret | Used by | Notes |
| --- | --- | --- |
| `APPLE_ID` | TestFlight | App Store Connect login email |
| `APPLE_APP_SPECIFIC_PASSWORD` | TestFlight | Optional fallback for password auth |
| `ITC_TEAM_ID` | TestFlight | App Store Connect team id |
| `TEAM_ID` | TestFlight | Developer Portal team id |
| `MATCH_PASSWORD` | TestFlight | Passphrase that decrypts the match storage repo |
| `MATCH_GIT_BASIC_AUTHORIZATION` | TestFlight | Base64-encoded basic-auth header for the match git remote |
| `APP_STORE_CONNECT_API_KEY_ID` | TestFlight | App Store Connect API key id (modern path) |
| `APP_STORE_CONNECT_API_ISSUER_ID` | TestFlight | App Store Connect API issuer id |
| `APP_STORE_CONNECT_API_KEY` | TestFlight | App Store Connect API key contents (`.p8` body) |
| `IRONFLYER_APP_IDENTIFIER` | TestFlight | Reverse-DNS bundle id (e.g. `com.ironflyer.starter`) |
