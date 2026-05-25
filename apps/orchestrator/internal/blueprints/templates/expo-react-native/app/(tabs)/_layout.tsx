import { Tabs } from "expo-router";

export default function TabsLayout() {
  return (
    <Tabs
      screenOptions={{
        tabBarActiveTintColor: "#c6ff00",
        tabBarStyle: { backgroundColor: "#0a0a0a", borderTopColor: "#1a1a1a" },
        headerStyle: { backgroundColor: "#0a0a0a" },
        headerTintColor: "#f5f1e6",
      }}
    >
      <Tabs.Screen name="profile" options={{ title: "Profile" }} />
    </Tabs>
  );
}
