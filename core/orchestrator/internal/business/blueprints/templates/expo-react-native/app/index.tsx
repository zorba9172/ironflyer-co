import { Link } from "expo-router";
import { StyleSheet, Text, View, Pressable } from "react-native";

export default function HomeScreen() {
  return (
    <View style={styles.container}>
      <Text style={styles.title}>Ironflyer Expo Starter</Text>
      <Text style={styles.subtitle}>
        Expo SDK 51 + expo-router + TypeScript. Edit{" "}
        <Text style={styles.code}>app/index.tsx</Text> to begin.
      </Text>
      <Link href="/(tabs)/profile" asChild>
        <Pressable style={styles.button}>
          <Text style={styles.buttonLabel}>Open profile tab</Text>
        </Pressable>
      </Link>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "#0a0a0a",
    alignItems: "center",
    justifyContent: "center",
    padding: 24,
  },
  title: {
    color: "#f5f1e6",
    fontSize: 28,
    fontWeight: "700",
    marginBottom: 12,
    textAlign: "center",
  },
  subtitle: {
    color: "#bdbdbd",
    fontSize: 16,
    textAlign: "center",
    marginBottom: 32,
  },
  code: {
    fontFamily: "Menlo",
    color: "#c6ff00",
  },
  button: {
    backgroundColor: "#c6ff00",
    paddingHorizontal: 24,
    paddingVertical: 12,
    borderRadius: 8,
  },
  buttonLabel: {
    color: "#0a0a0a",
    fontWeight: "700",
  },
});
