import { View, Text, Pressable, useColorScheme, StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { makeNativeTheme } from '@ironflyer/ui-native';

export default function Home() {
  const scheme = useColorScheme() ?? 'dark';
  const t = makeNativeTheme(scheme === 'light' ? 'light' : 'dark');

  return (
    <SafeAreaView style={[styles.root, { backgroundColor: t.color.bg }]}>
      <View style={styles.body}>
        <Text style={[styles.kicker, { color: t.color.secondary }]}>AI PRODUCT FINISHER</Text>
        <Text style={[styles.h1, { color: t.color.textPrimary }]}>Finish what your AI started.</Text>
        <Text style={[styles.sub, { color: t.color.textSecondary }]}>
          Track your projects, watch the finisher gates close, and ship — from your phone.
        </Text>

        <Pressable style={[styles.cta, { backgroundColor: t.color.primary }]}>
          <Text style={styles.ctaText}>Open a project</Text>
        </Pressable>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  body: { flex: 1, padding: 24, justifyContent: 'center', gap: 12 },
  kicker: { fontSize: 12, letterSpacing: 1.5, fontWeight: '600' },
  h1: { fontSize: 34, fontWeight: '700', lineHeight: 38 },
  sub: { fontSize: 16, lineHeight: 24 },
  cta: { marginTop: 16, paddingVertical: 14, borderRadius: 10, alignItems: 'center' },
  ctaText: { color: '#fff', fontWeight: '600', fontSize: 16 },
});
