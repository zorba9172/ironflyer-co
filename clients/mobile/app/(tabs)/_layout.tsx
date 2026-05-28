import { Tabs } from 'expo-router';
import { View, StyleSheet } from 'react-native';
import { useTheme } from '../../lib/theme';

// Dependency-free tab icon: a small rounded square that fills with the brand
// color when the tab is focused. Keeps us off undeclared icon-font deps while
// still reading as a native bottom tab.
function TabGlyph({ color, focused }: { color: string; focused: boolean }) {
  return (
    <View
      style={[
        styles.glyph,
        { borderColor: color, backgroundColor: focused ? color : 'transparent' },
      ]}
    />
  );
}

export default function TabsLayout() {
  const t = useTheme();
  return (
    <Tabs
      screenOptions={{
        headerStyle: { backgroundColor: t.color.bg },
        headerTintColor: t.color.textPrimary,
        headerTitleStyle: { fontWeight: '700' },
        sceneStyle: { backgroundColor: t.color.bg },
        tabBarStyle: {
          backgroundColor: t.color.surface,
          borderTopColor: t.color.border,
        },
        tabBarActiveTintColor: t.color.primary,
        tabBarInactiveTintColor: t.color.textMuted,
      }}
    >
      <Tabs.Screen
        name="index"
        options={{
          title: 'Home',
          tabBarIcon: ({ color, focused }) => <TabGlyph color={color} focused={focused} />,
        }}
      />
      <Tabs.Screen
        name="projects"
        options={{
          title: 'Projects',
          tabBarIcon: ({ color, focused }) => <TabGlyph color={color} focused={focused} />,
        }}
      />
    </Tabs>
  );
}

const styles = StyleSheet.create({
  glyph: { width: 18, height: 18, borderRadius: 6, borderWidth: 2 },
});
