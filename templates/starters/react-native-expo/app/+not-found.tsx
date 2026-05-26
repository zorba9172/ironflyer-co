import { Link, Stack } from 'expo-router'
import { StyleSheet, Text, View } from 'react-native'

const theme = {
  color: {
    bg: '#050507',
    text: '#e8e8ef',
    textMuted: '#9a9aae',
    link: '#8f4dff'
  }
} as const

export default function NotFound() {
  return (
    <>
      <Stack.Screen options={{ title: 'Not found' }} />
      <View style={styles.container}>
        <Text style={styles.code}>404</Text>
        <Text style={styles.title}>This screen does not exist.</Text>
        <Text style={styles.body}>The route you opened is not registered in this app.</Text>
        <Link href='/' style={styles.link}>
          Back to home
        </Link>
      </View>
    </>
  )
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: theme.color.bg,
    padding: 24,
    gap: 12
  },
  code: {
    color: theme.color.textMuted,
    fontSize: 48,
    fontWeight: '800',
    letterSpacing: 2
  },
  title: {
    color: theme.color.text,
    fontSize: 20,
    fontWeight: '700'
  },
  body: {
    color: theme.color.textMuted,
    fontSize: 14,
    textAlign: 'center'
  },
  link: {
    marginTop: 12,
    color: theme.color.link,
    fontSize: 15,
    fontWeight: '600'
  }
})
