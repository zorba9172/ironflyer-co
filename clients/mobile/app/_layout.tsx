import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { useColorScheme } from 'react-native';
import { makeNativeTheme } from '@ironflyer/ui-native';

export default function RootLayout() {
  const scheme = useColorScheme() ?? 'dark';
  const theme = makeNativeTheme(scheme === 'light' ? 'light' : 'dark');
  return (
    <>
      <StatusBar style={scheme === 'light' ? 'dark' : 'light'} />
      <Stack
        screenOptions={{
          headerStyle: { backgroundColor: theme.color.bg },
          headerTintColor: theme.color.textPrimary,
          contentStyle: { backgroundColor: theme.color.bg },
          headerTitleStyle: { fontWeight: '700' },
        }}
      >
        {/* The tab navigator owns its own header per screen. */}
        <Stack.Screen name="(tabs)" options={{ headerShown: false }} />
        {/* Project detail is pushed on top of the tabs. */}
        <Stack.Screen name="project/[id]" options={{ title: 'Project' }} />
      </Stack>
    </>
  );
}
