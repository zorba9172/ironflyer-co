# Expo + React Native starter (Ironflyer)

A production-discipline Expo Router starter, wired for the Ironflyer build
gates. Two screens, EAS profiles, typed routes, dark theme.

## Quick start

```bash
npm install
npx expo install --fix    # pin RN / Expo deps to the SDK matrix
npx expo prebuild         # generate native ios/ and android/ projects
npm run android           # or: npm run ios
```

To run the JS-only dev experience:

```bash
npm run start
```

## EAS Build

Three profiles are pre-configured in `eas.json`:

| Profile       | Distribution | Notes                                              |
| ------------- | ------------ | -------------------------------------------------- |
| `development` | internal     | Dev client, simulator-capable iOS, debug APK       |
| `preview`     | internal     | APK build for stakeholders, channel `preview`      |
| `production`  | store        | `autoIncrement: true`, channel `main`              |

Run a build:

```bash
npx eas build --profile preview --platform android
npx eas build --profile production --platform ios
```

Set `EAS_PROJECT_ID` in your environment (or `.env.local`) and `app.config.ts`
will pick it up automatically.

## Project structure

```
app/
  _layout.tsx       Root Stack, SafeAreaProvider, dark status bar
  index.tsx         Landing — gates-forward hero + CTA
  dashboard.tsx     Three cards (Builds, Crashes, Analytics)
  +not-found.tsx    404 fallback
assets/             Drop icon.png, splash.png, adaptive-icon.png, favicon.png here
app.json            Expo manifest
app.config.ts       Typed extension that injects EAS_PROJECT_ID
eas.json            Build profiles
```

## What Ironflyer adds

- **Bundle identifier gate.** `com.ironflyer.starter` is rejected by the build
  gate until you change it in both `ios.bundleIdentifier` and `android.package`.
- **Signing-secret validation.** EAS credentials (keystore, provisioning
  profile, App Store Connect key) are checked before a build is queued, so
  you do not pay for a build that will fail at upload.
- **ProfitGuard runtime metering.** Every minute the Android emulator or iOS
  simulator spends running under Ironflyer is metered against your workspace
  budget. Idle simulators get shut down automatically.

## Editing this starter

- TypeScript strict mode is on. Path alias `@/*` resolves from the project root.
- The header is transparent on iOS, solid on Android — see `_layout.tsx`.
- Colors are defined inline at the top of each screen as a `theme` const so the
  starter does not depend on a design-tokens package that does not exist in
  user workspaces.

## Replacing placeholder assets

See `assets/README.md` for sizes and an ImageMagick one-liner to unblock the
first prebuild.

## CI/CD

GitHub Actions workflows ship with this starter under `.github/workflows/`:

- [`eas-build.yml`](.github/workflows/eas-build.yml) — Android + iOS
  binaries via EAS. Triggered on push to `main` (preview profile) and
  manually for `development` / `preview` / `production`.
- [`eas-submit.yml`](.github/workflows/eas-submit.yml) — manual upload of
  a finished EAS build to Google Play or TestFlight. Takes a build ID +
  platform.
- [`eas-update.yml`](.github/workflows/eas-update.yml) — over-the-air
  JS-bundle updates against the `preview` channel on every push to
  `main`.

### Required GitHub secrets

| Secret | Used by | Notes |
| --- | --- | --- |
| `EXPO_TOKEN` | every workflow | Personal access token from `expo.dev` |
