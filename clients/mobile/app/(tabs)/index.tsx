import { View, Text, Pressable, StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useRouter } from 'expo-router';
import { useTheme } from '../../lib/theme';
import { useProjects } from '../../hooks/useProjects';

export default function Home() {
  const t = useTheme();
  const router = useRouter();
  const { data: projects } = useProjects();

  const total = projects.length;
  const shippable = projects.filter((p) => p.gates.every((g) => g.status === 'closed')).length;

  return (
    <SafeAreaView style={[styles.root, { backgroundColor: t.color.bg }]} edges={['bottom']}>
      <View style={[styles.body, { padding: t.space[5], gap: t.space[3] }]}>
        <Text style={[styles.kicker, { color: t.color.secondary }]}>AI PRODUCT FINISHER</Text>
        <Text style={[styles.h1, { color: t.color.textPrimary }]}>Finish what your AI started.</Text>
        <Text style={[styles.sub, { color: t.color.textSecondary }]}>
          Track your projects, watch the finisher gates close, and ship — from your phone.
        </Text>

        {/* Live snapshot card sourced from sample data. */}
        <View
          style={[
            styles.statCard,
            {
              backgroundColor: t.color.surface,
              borderColor: t.color.border,
              borderRadius: t.radius.lg,
              padding: t.space[4],
              marginTop: t.space[4],
            },
          ]}
        >
          <View style={styles.statRow}>
            <View style={styles.stat}>
              <Text style={[styles.statValue, { color: t.color.textPrimary }]}>{total}</Text>
              <Text style={[styles.statLabel, { color: t.color.textMuted }]}>Projects</Text>
            </View>
            <View style={[styles.divider, { backgroundColor: t.color.border }]} />
            <View style={styles.stat}>
              <Text style={[styles.statValue, { color: t.color.success }]}>{shippable}</Text>
              <Text style={[styles.statLabel, { color: t.color.textMuted }]}>Shippable</Text>
            </View>
            <View style={[styles.divider, { backgroundColor: t.color.border }]} />
            <View style={styles.stat}>
              <Text style={[styles.statValue, { color: t.color.signal }]}>{total - shippable}</Text>
              <Text style={[styles.statLabel, { color: t.color.textMuted }]}>In progress</Text>
            </View>
          </View>
        </View>

        <Pressable
          onPress={() => router.push('/projects')}
          style={({ pressed }) => [
            styles.cta,
            {
              backgroundColor: t.color.primary,
              borderRadius: t.radius.md,
              paddingVertical: t.space[3] + 2,
              marginTop: t.space[4],
              opacity: pressed ? 0.85 : 1,
            },
          ]}
        >
          <Text style={[styles.ctaText, { color: t.color.textPrimary }]}>Open a project</Text>
        </Pressable>
      </View>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  body: { flex: 1, justifyContent: 'center' },
  kicker: { fontSize: 12, letterSpacing: 1.5, fontWeight: '600' },
  h1: { fontSize: 34, fontWeight: '700', lineHeight: 38 },
  sub: { fontSize: 16, lineHeight: 24 },
  statCard: { borderWidth: 1 },
  statRow: { flexDirection: 'row', alignItems: 'center' },
  stat: { flex: 1, alignItems: 'center', gap: 4 },
  statValue: { fontSize: 26, fontWeight: '700' },
  statLabel: { fontSize: 12, fontWeight: '500' },
  divider: { width: 1, alignSelf: 'stretch', marginVertical: 4 },
  cta: { alignItems: 'center' },
  ctaText: { fontWeight: '600', fontSize: 16 },
});
