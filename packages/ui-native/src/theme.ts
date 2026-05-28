import { brand, modes, type ThemeMode } from '@ironflyer/design-tokens/brand';

// Native theme object derived from the same brand tokens as the web MUI theme,
// so the two platforms stay in lockstep.
export function makeNativeTheme(mode: ThemeMode) {
  const m = modes[mode];
  return {
    mode,
    color: {
      bg: m.bg,
      surface: m.surface,
      surfaceRaised: m.surfaceRaised,
      border: m.borderSubtle,
      textPrimary: m.textPrimary,
      textSecondary: m.textSecondary,
      textMuted: m.textMuted,
      primary: brand.accent.primary,
      secondary: brand.accent.secondary,
      signal: brand.accent.signal,
      success: brand.accent.success,
      danger: brand.accent.danger,
    },
    radius: brand.radius,
    space: brand.space,
    font: brand.typography,
  } as const;
}

export type NativeTheme = ReturnType<typeof makeNativeTheme>;
