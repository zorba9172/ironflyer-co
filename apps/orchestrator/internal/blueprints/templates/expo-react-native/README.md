# Expo React Native Blueprint

Expo SDK 51 + expo-router + TypeScript. Cross-platform mobile (iOS,
Android, web) starter with a tab bar and a typed home screen.

## Quick start

```bash
npm install
npm run start    # opens Expo Dev Tools
npm run ios      # iOS simulator (macOS only)
npm run android  # Android emulator
npm run web      # browser preview
```

Then scan the QR code with the Expo Go app on a physical device.

## Structure

```
app/
  _layout.tsx        Stack root + SafeAreaProvider
  index.tsx          Home screen
  (tabs)/
    _layout.tsx      Bottom tab bar
    profile.tsx      Profile tab with a stateful text input
assets/
  placeholder.txt    Where to drop your icon.png
```

## Adding screens

Drop a `*.tsx` file inside `app/`. expo-router auto-creates the route
from the file path. Use `Link` (`expo-router`) for navigation.

## Icon

This blueprint ships no binary assets. See `assets/placeholder.txt`
for the icon convention before running `expo prebuild` or submitting
to the App Store / Play Store.

## Typecheck

```bash
npm run typecheck
```
