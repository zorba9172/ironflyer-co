# Ironflyer Android Starter

A Kotlin + Jetpack Compose starter wired for the Ironflyer build gates.

## Prerequisites

- Android SDK 35 (with platform-tools)
- JDK 17
- Gradle 8.11.1 wrapper (regenerate with `gradle wrapper --gradle-version 8.11.1` or let Android Studio do it on first open)

## Build

```sh
./gradlew :app:assembleDebug
```

## Install on a connected device or emulator

```sh
./gradlew :app:installDebug
```

## What Ironflyer adds on top of a vanilla Compose starter

- Gate-enforced `applicationId` — promotion fails if the package name diverges from the registered project identity.
- Signing secrets are validated by the finisher before a `release` build is allowed to ship.
- ProfitGuard meters emulator minutes against the project budget so CI runs cannot silently burn margin.

## CI/CD

GitHub Actions workflows ship with this starter under `.github/workflows/`:

- [`android-build.yml`](.github/workflows/android-build.yml) — assembles
  a debug APK on every push to `main` and on manual triggers. Uploads
  the APK as an artifact for sideloading.
- [`android-release.yml`](.github/workflows/android-release.yml) —
  manual-only. Decodes a release keystore from secrets, runs
  `gradle bundleRelease`, and uploads the AAB to the Play Store
  internal track via [`r0adkll/upload-google-play`](https://github.com/r0adkll/upload-google-play).

Fastlane lanes live under [`fastlane/`](fastlane/):

- `bundle exec fastlane debug` — produces a sideloadable debug APK.
- `bundle exec fastlane release` — signs and submits an AAB to the Play
  Store internal track as a draft.

### Required GitHub secrets

| Secret | Used by | Notes |
| --- | --- | --- |
| `ANDROID_KEYSTORE_BASE64` | `android-release.yml` | Base64-encoded `.keystore` blob |
| `ANDROID_KEYSTORE_PASSWORD` | `android-release.yml` | Decryption password for the keystore |
| `ANDROID_KEY_ALIAS` | `android-release.yml` | Signing-key alias inside the keystore |
| `ANDROID_KEY_PASSWORD` | `android-release.yml` | Signing-key password |
| `GOOGLE_PLAY_SERVICE_ACCOUNT_KEY` | `android-release.yml` | JSON service-account key for the Play Developer API |
