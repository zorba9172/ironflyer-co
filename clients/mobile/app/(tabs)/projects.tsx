import { View, Text, Pressable, FlatList, StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useRouter } from 'expo-router';
import { useTheme, type NativeTheme } from '../../lib/theme';
import {
  projects,
  openGateCount,
  shippablePercent,
  type Project,
} from '../../lib/sampleData';

function ProgressBar({ percent, theme }: { percent: number; theme: NativeTheme }) {
  const color = percent === 100 ? theme.color.success : theme.color.primary;
  return (
    <View
      style={[
        styles.track,
        { backgroundColor: theme.color.border, borderRadius: theme.radius.pill },
      ]}
    >
      <View
        style={{
          width: `${percent}%`,
          height: '100%',
          backgroundColor: color,
          borderRadius: theme.radius.pill,
        }}
      />
    </View>
  );
}

function ProjectCard({ project }: { project: Project }) {
  const t = useTheme();
  const router = useRouter();
  const percent = shippablePercent(project);
  const open = openGateCount(project);

  return (
    <Pressable
      onPress={() => router.push(`/project/${project.id}`)}
      style={({ pressed }) => [
        styles.card,
        {
          backgroundColor: t.color.surface,
          borderColor: t.color.border,
          borderRadius: t.radius.lg,
          padding: t.space[4],
          opacity: pressed ? 0.9 : 1,
        },
      ]}
    >
      <Text style={[styles.cardTitle, { color: t.color.textPrimary }]}>{project.name}</Text>
      <Text style={[styles.cardSource, { color: t.color.secondary, marginTop: t.space[1] }]}>
        {project.source}
      </Text>
      <Text style={[styles.cardSummary, { color: t.color.textSecondary, marginTop: t.space[2] }]}>
        {project.summary}
      </Text>

      <View style={[styles.metaRow, { marginTop: t.space[4] }]}>
        <Text style={[styles.meta, { color: t.color.textMuted }]}>
          {open === 0 ? 'All gates closed' : `${open} gate${open === 1 ? '' : 's'} open`}
        </Text>
        <Text
          style={[
            styles.percent,
            { color: percent === 100 ? t.color.success : t.color.textPrimary },
          ]}
        >
          {percent}% to shippable
        </Text>
      </View>
      <View style={{ marginTop: t.space[2] }}>
        <ProgressBar percent={percent} theme={t} />
      </View>
    </Pressable>
  );
}

export default function Projects() {
  const t = useTheme();
  return (
    <SafeAreaView style={[styles.root, { backgroundColor: t.color.bg }]} edges={['bottom']}>
      <FlatList
        data={projects}
        keyExtractor={(p) => p.id}
        renderItem={({ item }) => <ProjectCard project={item} />}
        contentContainerStyle={{ padding: t.space[4], gap: t.space[3] }}
        ListHeaderComponent={
          <Text style={[styles.header, { color: t.color.textSecondary, marginBottom: t.space[2] }]}>
            Tap a project to inspect its finisher gates.
          </Text>
        }
      />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  header: { fontSize: 14 },
  card: { borderWidth: 1 },
  cardTitle: { fontSize: 18, fontWeight: '700' },
  cardSource: { fontSize: 12, fontWeight: '600', letterSpacing: 0.4 },
  cardSummary: { fontSize: 14, lineHeight: 20 },
  metaRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  meta: { fontSize: 13, fontWeight: '500' },
  percent: { fontSize: 13, fontWeight: '700' },
  track: { height: 6, width: '100%', overflow: 'hidden' },
});
