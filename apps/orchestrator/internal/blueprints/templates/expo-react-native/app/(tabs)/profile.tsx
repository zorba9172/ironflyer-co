import { useState } from "react";
import { StyleSheet, Text, TextInput, View } from "react-native";

export default function ProfileScreen() {
  const [name, setName] = useState("");

  return (
    <View style={styles.container}>
      <Text style={styles.label}>Display name</Text>
      <TextInput
        value={name}
        onChangeText={setName}
        placeholder="Enter your name"
        placeholderTextColor="#777"
        style={styles.input}
        autoCapitalize="words"
      />
      <Text style={styles.greeting}>
        {name ? `Hello, ${name}.` : "Enter a name to see a greeting."}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: "#0a0a0a",
    padding: 24,
  },
  label: {
    color: "#f5f1e6",
    fontSize: 14,
    marginBottom: 8,
    textTransform: "uppercase",
    letterSpacing: 1.2,
  },
  input: {
    backgroundColor: "#141414",
    borderColor: "#2a2a2a",
    borderWidth: 1,
    borderRadius: 8,
    color: "#f5f1e6",
    paddingHorizontal: 12,
    paddingVertical: 10,
    fontSize: 16,
    marginBottom: 24,
  },
  greeting: {
    color: "#c6ff00",
    fontSize: 18,
    fontWeight: "600",
  },
});
