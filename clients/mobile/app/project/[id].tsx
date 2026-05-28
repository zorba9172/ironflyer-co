import { View, Text, FlatList, StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { Stack, useLocalSearchParams } from 'expo-router';
import { useTheme } from '../../lib/theme';
import { getProject, shippablePercent, type Gate } from '../../lib/sampleData';
import { gateStatusColor, gateStatusLabel } from '../../lib/gateStatus';

function GateRow({ gate }: { gate: Gate }) {
  const t = useTheme();
  const color = gateStatusColor(t, gate.status);
  return (
    <View
      style={[
        styles.gate,
        {
          backgroundColor: t.color.surface,
          borderColor: t.color.border,
          borderRadius: t.radius.md,
          padding: t.space[4],
        },
      ]}
    >
      <View style={styles.gateHead}>
        <View style={styles.gateNameWrap}>
          <View style={[styles.dot, { backgroundColor: color }]} />
          <Text style={[styles.gateName, { color: t.color.textPrimary }]}>{gate.name}</Text>
        </View>
        <View
          style={[
            styles.badge,
            { borderColor: color, borderRadius: t.radius.pill, paddingHorizontal: t.space[2] },
          ]}
        >
          <Text style={[styles.badgeText, { color }]}>{gateStatusLabel(gate.status)}</Text>
        </View>
      </View>
      {gate.blocking ? (
        <Text style={[styles.blocking, { color: t.color.textSecondary, marginTop: t.space[2] }]}>
          {gate.blocking}
        </Text>
      ) : null}
    </View>
  );
}

export default function ProjectDetail() {
  const t = useTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const project = id ? getProject(id) : undefined;

  if (!project) {
    return (
      <SafeAreaView style={[styles.root, { backgroundColor: t.color.bg }]} edges={['bottom']}>
        <Stack.Screen options={{ title: 'Not found' }} />
        <View style={[styles.empty, { padding: t.space[5] }]}>
          <Text style={[styles.emptyText, { color: t.color.textSecondary }]}>
            That project could not be found.
          </Text>
        </View>
      </SafeAreaView>
    );
  }

  const percent = shippablePercent(project);

  return (
    <SafeAreaView style={[styles.root, { backgroundColor: t.color.bg }]} edges={['bottom']}>
      <Stack.Screen options={{ title: project.name }} />
      <FlatList
        data={project.gates}
        keyExtractor={(g) => g.id}
        renderItem={({ item }) => <GateRow gate={item} />}
        contentContainerStyle={{ padding: t.space[4], gap: t.space[3] }}
        ListHeaderComponent={
          <View style={{ marginBottom: t.space[2] }}>
            <Text style={[styles.source, { color: t.color.secondary }]}>{project.source}</Text>
            <Text style={[styles.summary, { color: t.color.textSecondary, marginTop: t.space[2] }]}>
              {project.summary}
            </Text>
            <Text
              style={[
                styles.progress,
                {
                  color: percent === 100 ? t.color.success : t.color.textPrimary,
                  marginTop: t.space[3],
                },
              ]}
            >
              {percent}% to shippable
            </Text>
            <Text
              style={[styles.sectionLabel, { color: t.color.textMuted, marginTop: t.space[4] }]}
            >
              FINISHER GATES
            </Text>
          </View>
        }
      />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  empty: { flex: 1, justifyContent: 'center', alignItems: 'center' },
  emptyText: { fontSize: 16 },
  source: { fontSize: 12, fontWeight: '600', letterSpacing: 0.4 },
  summary: { fontSize: 15, lineHeight: 22 },
  progress: { fontSize: 16, fontWeight: '700' },
  sectionLabel: { fontSize: 12, fontWeight: '600', letterSpacing: 1.2 },
  gate: { borderWidth: 1 },
  gateHead: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  gateNameWrap: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  dot: { width: 10, height: 10, borderRadius: 5 },
  gateName: { fontSize: 16, fontWeight: '600' },
  badge: { borderWidth: 1, paddingVertical: 2 },
  badgeText: { fontSize: 11, fontWeight: '700' },
  blocking: { fontSize: 13, lineHeight: 19 },
});
