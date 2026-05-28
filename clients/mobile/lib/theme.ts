import { useColorScheme } from 'react-native';
import { makeNativeTheme, type NativeTheme } from '@ironflyer/ui-native';

// Single place that resolves the active color scheme into the shared native
// brand theme. Every screen reads colors/spacing/fonts from here so nothing is
// hardcoded outside of @ironflyer/design-tokens.
export function useTheme(): NativeTheme {
  const scheme = useColorScheme() ?? 'dark';
  return makeNativeTheme(scheme === 'light' ? 'light' : 'dark');
}

export { type NativeTheme } from '@ironflyer/ui-native';
