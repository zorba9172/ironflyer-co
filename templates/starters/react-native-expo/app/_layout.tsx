import { Stack } from 'expo-router'
import { StatusBar } from 'expo-status-bar'
import { Platform } from 'react-native'
import { SafeAreaProvider } from 'react-native-safe-area-context'

export default function RootLayout() {
  return (
    <SafeAreaProvider>
      <StatusBar style='light' />
      <Stack
        screenOptions={{
          headerStyle: {
            backgroundColor: Platform.OS === 'ios' ? 'transparent' : '#050507'
          },
          headerTransparent: Platform.OS === 'ios',
          headerTintColor: '#e8e8ef',
          headerTitleStyle: {
            fontWeight: '600'
          },
          contentStyle: {
            backgroundColor: '#050507'
          }
        }}
      >
        <Stack.Screen name='index' options={{ headerShown: false }} />
        <Stack.Screen name='dashboard' options={{ title: 'Dashboard' }} />
        <Stack.Screen name='+not-found' options={{ title: 'Not found' }} />
      </Stack>
    </SafeAreaProvider>
  )
}
