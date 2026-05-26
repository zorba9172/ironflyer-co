import Constants from 'expo-constants'
import { LinearGradient } from 'expo-linear-gradient'
import { Link } from 'expo-router'
import {
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View
} from 'react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'

const theme = {
  color: {
    bg: '#050507',
    surface: '#0f0f17',
    surfaceMuted: '#15151f',
    border: '#23232f',
    text: '#e8e8ef',
    textMuted: '#9a9aae',
    primary: '#8f4dff',
    mint: '#4dffb0',
    coral: '#ff4d4d',
    magenta: '#ff4dd1',
    purple: '#8f4dff'
  },
  radius: {
    sm: 8,
    md: 14,
    lg: 22
  },
  space: {
    xs: 6,
    sm: 10,
    md: 16,
    lg: 24,
    xl: 36
  },
  font: {
    h1: 34,
    h2: 22,
    body: 15,
    label: 13
  }
} as const

export default function Home() {
  const insets = useSafeAreaInsets()
  const projectName = Constants.expoConfig?.name ?? 'Ironflyer Starter'

  return (
    <ScrollView
      style={styles.root}
      contentContainerStyle={[
        styles.container,
        { paddingTop: insets.top + theme.space.lg, paddingBottom: insets.bottom + theme.space.xl }
      ]}
    >
      <View style={styles.header}>
        <View style={styles.badge}>
          <View style={styles.badgeDot} />
          <Text style={styles.badgeText}>Production-discipline mobile</Text>
        </View>
        <Text style={styles.title}>{projectName}</Text>
        <Text style={styles.subtitle}>
          Every screen ships through a gate. Bundle identifiers, signing keys, store metadata, and crash budgets are
          validated before a build can leave your laptop.
        </Text>
      </View>

      <View style={styles.heroCard}>
        <LinearGradient
          colors={[theme.color.coral, theme.color.magenta, theme.color.purple]}
          start={{ x: 0, y: 0 }}
          end={{ x: 1, y: 1 }}
          style={styles.heroGradient}
        >
          <Text style={styles.heroEyebrow}>Gates forward</Text>
          <Text style={styles.heroHeadline}>Close the loop end to end.</Text>
          <Text style={styles.heroBody}>
            Ironflyer names what is unclosed: missing icons, unsigned builds, untracked crashes. Open the dashboard to
            see the state of your app at a glance.
          </Text>
        </LinearGradient>

        <View style={styles.heroFooter}>
          <Link href='/dashboard' asChild>
            <Pressable style={({ pressed }) => [styles.cta, pressed && styles.ctaPressed]}>
              <LinearGradient
                colors={[theme.color.coral, theme.color.magenta, theme.color.purple]}
                start={{ x: 0, y: 0 }}
                end={{ x: 1, y: 0 }}
                style={styles.ctaGradient}
              >
                <Text style={styles.ctaText}>Open dashboard</Text>
              </LinearGradient>
            </Pressable>
          </Link>
          <Text style={styles.heroHint}>
            Next gate: replace placeholder icons in <Text style={styles.code}>assets/</Text>.
          </Text>
        </View>
      </View>

      <View style={styles.metaRow}>
        <Meta label='Runtime' value='Expo SDK 53' />
        <Meta label='Router' value='expo-router 4' />
        <Meta label='Arch' value='New Arch' />
      </View>
    </ScrollView>
  )
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <View style={styles.meta}>
      <Text style={styles.metaLabel}>{label}</Text>
      <Text style={styles.metaValue}>{value}</Text>
    </View>
  )
}

const styles = StyleSheet.create({
  root: {
    flex: 1,
    backgroundColor: theme.color.bg
  },
  container: {
    paddingHorizontal: theme.space.lg,
    gap: theme.space.lg
  },
  header: {
    gap: theme.space.md
  },
  badge: {
    alignSelf: 'flex-start',
    flexDirection: 'row',
    alignItems: 'center',
    gap: theme.space.xs,
    paddingHorizontal: theme.space.sm,
    paddingVertical: 6,
    borderRadius: 999,
    borderWidth: 1,
    borderColor: theme.color.border,
    backgroundColor: theme.color.surface
  },
  badgeDot: {
    width: 6,
    height: 6,
    borderRadius: 999,
    backgroundColor: theme.color.mint
  },
  badgeText: {
    color: theme.color.textMuted,
    fontSize: theme.font.label,
    letterSpacing: 0.3
  },
  title: {
    color: theme.color.text,
    fontSize: theme.font.h1,
    fontWeight: '700',
    letterSpacing: -0.5
  },
  subtitle: {
    color: theme.color.textMuted,
    fontSize: theme.font.body,
    lineHeight: 22
  },
  heroCard: {
    borderRadius: theme.radius.lg,
    overflow: 'hidden',
    backgroundColor: theme.color.surface,
    borderWidth: 1,
    borderColor: theme.color.border
  },
  heroGradient: {
    padding: theme.space.lg,
    gap: theme.space.sm
  },
  heroEyebrow: {
    color: '#1a0a14',
    fontSize: theme.font.label,
    fontWeight: '700',
    letterSpacing: 1.2,
    textTransform: 'uppercase'
  },
  heroHeadline: {
    color: '#10000a',
    fontSize: 26,
    fontWeight: '800',
    letterSpacing: -0.4
  },
  heroBody: {
    color: '#1f0010',
    fontSize: theme.font.body,
    lineHeight: 22
  },
  heroFooter: {
    padding: theme.space.lg,
    gap: theme.space.md
  },
  cta: {
    borderRadius: theme.radius.md,
    overflow: 'hidden'
  },
  ctaPressed: {
    opacity: 0.85
  },
  ctaGradient: {
    paddingVertical: 14,
    paddingHorizontal: theme.space.lg,
    alignItems: 'center'
  },
  ctaText: {
    color: '#ffffff',
    fontSize: 16,
    fontWeight: '700',
    letterSpacing: 0.2
  },
  heroHint: {
    color: theme.color.textMuted,
    fontSize: theme.font.label
  },
  code: {
    fontFamily: 'Courier',
    color: theme.color.text
  },
  metaRow: {
    flexDirection: 'row',
    gap: theme.space.sm
  },
  meta: {
    flex: 1,
    padding: theme.space.md,
    borderRadius: theme.radius.md,
    backgroundColor: theme.color.surfaceMuted,
    borderWidth: 1,
    borderColor: theme.color.border,
    gap: 4
  },
  metaLabel: {
    color: theme.color.textMuted,
    fontSize: theme.font.label,
    textTransform: 'uppercase',
    letterSpacing: 0.6
  },
  metaValue: {
    color: theme.color.text,
    fontSize: theme.font.body,
    fontWeight: '600'
  }
})
