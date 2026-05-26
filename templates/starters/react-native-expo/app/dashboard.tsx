import { FlatList, StyleSheet, Text, View } from 'react-native'

const theme = {
  color: {
    bg: '#050507',
    surface: '#0f0f17',
    border: '#23232f',
    text: '#e8e8ef',
    textMuted: '#9a9aae',
    mint: '#4dffb0',
    coral: '#ff4d4d',
    amber: '#ffb74d'
  },
  radius: { md: 14 },
  space: { sm: 10, md: 16, lg: 24 }
} as const

type Card = {
  id: string
  title: string
  description: string
  metric: string
  trend: 'up' | 'down' | 'flat'
  accent: string
}

const cards: Card[] = [
  {
    id: 'builds',
    title: 'Builds',
    description: 'EAS pipeline status across development, preview, and production channels.',
    metric: '0 active',
    trend: 'flat',
    accent: theme.color.mint
  },
  {
    id: 'crashes',
    title: 'Crashes',
    description: 'Unhandled JS and native exceptions reported in the last 24 hours.',
    metric: '0 today',
    trend: 'flat',
    accent: theme.color.coral
  },
  {
    id: 'analytics',
    title: 'Analytics',
    description: 'Sessions, retention, and gate-pass rates streamed from your workspace.',
    metric: 'No data yet',
    trend: 'flat',
    accent: theme.color.amber
  }
]

export default function Dashboard() {
  return (
    <FlatList
      style={styles.root}
      contentContainerStyle={styles.container}
      data={cards}
      keyExtractor={(item) => item.id}
      ItemSeparatorComponent={() => <View style={{ height: theme.space.md }} />}
      ListHeaderComponent={
        <View style={styles.header}>
          <Text style={styles.eyebrow}>Workspace</Text>
          <Text style={styles.title}>Dashboard</Text>
          <Text style={styles.subtitle}>What is closed, what is open, what is bleeding budget.</Text>
        </View>
      }
      renderItem={({ item }) => (
        <View style={styles.card}>
          <View style={[styles.accent, { backgroundColor: item.accent }]} />
          <View style={styles.cardBody}>
            <View style={styles.cardHeader}>
              <Text style={styles.cardTitle}>{item.title}</Text>
              <Text style={styles.cardMetric}>{item.metric}</Text>
            </View>
            <Text style={styles.cardDescription}>{item.description}</Text>
          </View>
        </View>
      )}
    />
  )
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: theme.color.bg
  },
  container: {
    padding: theme.space.lg,
    paddingBottom: theme.space.lg * 2
  },
  header: {
    marginBottom: theme.space.lg,
    gap: 6
  },
  eyebrow: {
    color: theme.color.textMuted,
    fontSize: 12,
    textTransform: 'uppercase',
    letterSpacing: 1.2
  },
  title: {
    color: theme.color.text,
    fontSize: 28,
    fontWeight: '700',
    letterSpacing: -0.4
  },
  subtitle: {
    color: theme.color.textMuted,
    fontSize: 14,
    lineHeight: 20
  },
  card: {
    flexDirection: 'row',
    backgroundColor: theme.color.surface,
    borderRadius: theme.radius.md,
    borderWidth: 1,
    borderColor: theme.color.border,
    overflow: 'hidden'
  },
  accent: {
    width: 4
  },
  cardBody: {
    flex: 1,
    padding: theme.space.md,
    gap: theme.space.sm
  },
  cardHeader: {
    flexDirection: 'row',
    alignItems: 'baseline',
    justifyContent: 'space-between'
  },
  cardTitle: {
    color: theme.color.text,
    fontSize: 18,
    fontWeight: '700'
  },
  cardMetric: {
    color: theme.color.textMuted,
    fontSize: 13,
    fontWeight: '500'
  },
  cardDescription: {
    color: theme.color.textMuted,
    fontSize: 14,
    lineHeight: 20
  }
})
