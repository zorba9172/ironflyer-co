// MobileScaffolder — Expo Router (React Native) starter. Mirrors the
// shape of GameScaffolder / EcommerceScaffolder: a deterministic set of
// files + a contract markdown that the Coder reads as context.

package finisher

import (
	"context"
	"strings"

	"ironflyer/apps/orchestrator/internal/domain"
)

type MobileScaffolder struct{}

func (MobileScaffolder) Name() string { return "mobile-expo" }

func (MobileScaffolder) Applies(p *domain.Project) bool {
	if p == nil {
		return false
	}
	stack := strings.ToLower(p.Spec.Stack.Frontend + " " + p.Spec.Stack.Backend)
	if strings.Contains(stack, "react native") || strings.Contains(stack, "expo") ||
		strings.Contains(stack, "mobile") {
		return true
	}
	desc := strings.ToLower(p.Description + " " + p.Spec.Idea)
	if strings.Contains(desc, "mobile app") || strings.Contains(desc, "native app") ||
		strings.Contains(desc, "ios app") || strings.Contains(desc, "android app") {
		return true
	}
	for _, s := range p.Spec.UserStories {
		body := strings.ToLower(s.IWant + " " + s.SoThat + " " + strings.Join(s.Acceptance, " "))
		if strings.Contains(body, "ios") || strings.Contains(body, "android") ||
			strings.Contains(body, "push notification") || strings.Contains(body, "native app") ||
			strings.Contains(body, "mobile app") {
			return true
		}
	}
	return false
}

func (MobileScaffolder) Scaffold(_ context.Context, p *domain.Project) (DomainScaffold, error) {
	appName := "Ironflyer Mobile"
	if p != nil && strings.TrimSpace(p.Name) != "" {
		appName = p.Name
	}
	files := map[string]string{
		"app.config.ts": `// Expo app config — read by EAS, dev client, and metro at startup.
// Adjust slug + bundle identifiers before submitting to the stores.
import type { ExpoConfig } from 'expo/config';

const config: ExpoConfig = {
  name: '` + appName + `',
  slug: 'ironflyer-mobile',
  version: '0.1.0',
  orientation: 'portrait',
  scheme: 'ironflyer',
  userInterfaceStyle: 'automatic',
  newArchEnabled: true,
  ios: {
    supportsTablet: true,
    bundleIdentifier: 'com.ironflyer.mobile',
  },
  android: {
    package: 'com.ironflyer.mobile',
    adaptiveIcon: { backgroundColor: '#0d0e0f' },
  },
  plugins: ['expo-router'],
  experiments: { typedRoutes: true },
};

export default config;
`,
		"App.tsx": `// Expo Router root entry. The actual screens live under /app — this
// file only re-exports the router so the Metro bundler has a stable
// entrypoint that matches the default Expo template.
import 'expo-router/entry';
export {};
`,
		"app/_layout.tsx": `// Root layout. Wraps every route in a Stack navigator; the (tabs)
// group below it becomes the home experience. Add global providers
// (theme, query client, auth) inside this component.
import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';

export default function RootLayout() {
  return (
    <>
      <StatusBar style="light" />
      <Stack screenOptions={{ headerShown: false }}>
        <Stack.Screen name="(tabs)" />
      </Stack>
    </>
  );
}
`,
		"app/(tabs)/_layout.tsx": `// Tab navigator. Each file under /app/(tabs) becomes a tab; rename
// the route segment to change the URL. Use expo-router's <Tabs />
// instead of react-navigation directly so deep links keep working.
import { Tabs } from 'expo-router';

export default function TabsLayout() {
  return (
    <Tabs
      screenOptions={{
        tabBarActiveTintColor: '#c7ff00',
        tabBarStyle: { backgroundColor: '#0d0e0f', borderTopColor: '#1a1b1d' },
        headerStyle: { backgroundColor: '#0d0e0f' },
        headerTintColor: '#ffffff',
      }}
    >
      <Tabs.Screen name="index" options={{ title: 'Home' }} />
    </Tabs>
  );
}
`,
		"app/(tabs)/index.tsx": `// Home screen — first screen the user lands on after launch.
// Replace the body with your real product surface; the file path
// determines the route (expo-router file-based routing).
import { View, Text, StyleSheet } from 'react-native';

export default function HomeScreen() {
  return (
    <View style={styles.root}>
      <Text style={styles.title}>` + appName + `</Text>
      <Text style={styles.body}>
        Welcome. Edit app/(tabs)/index.tsx to start building your app.
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: '#0d0e0f',
    alignItems: 'center',
    justifyContent: 'center',
    padding: 24,
  },
  title: {
    color: '#c7ff00',
    fontSize: 28,
    fontWeight: '700',
    marginBottom: 12,
  },
  body: {
    color: '#ffffff',
    fontSize: 16,
    textAlign: 'center',
    lineHeight: 22,
  },
});
`,
	}
	contract := `Mobile scaffold: Expo Router (React Native, TypeScript).

Already provisioned:
- /app.config.ts            → Expo config; bundle ids + plugins
- /App.tsx                  → expo-router/entry shim
- /app/_layout.tsx          → root Stack navigator
- /app/(tabs)/_layout.tsx   → bottom tab bar
- /app/(tabs)/index.tsx     → home screen

How navigation works:
- File-based routing under /app/. Files become routes; folders become
  segments; (parens) are grouping segments (no URL segment).
- Use <Stack.Screen /> or <Tabs.Screen /> to customise headers/tabs.

Adding native capabilities:
- Camera / location / push: install the matching expo-* package
  (expo-camera, expo-location, expo-notifications) — they ship a
  config plugin so a managed-workflow build picks them up via
  app.config.ts. Do NOT eject unless absolutely required.

Package.json hint (do NOT clobber an existing one — merge instead):

    "dependencies": {
      "expo": "^54.0.0",
      "expo-router": "^4.0.0",
      "expo-status-bar": "^2.0.0",
      "react": "18.3.1",
      "react-native": "0.76.0",
      "react-native-safe-area-context": "^4.10.0",
      "react-native-screens": "^4.0.0"
    },
    "main": "expo-router/entry"

Run:    npx expo start
Build:  npx eas build --platform ios|android (requires EAS login)
`
	return DomainScaffold{Files: files, Contract: contract}, nil
}
