<script setup lang="ts">
import { useCounterStore } from "@/stores/counter";

const counter = useCounterStore();
</script>

<template>
  <section>
    <h1>Home</h1>
    <p>
      Vue 3 + Pinia + Vue Router + Vite. Edit
      <code>src/views/HomeView.vue</code> to customise.
    </p>
    <p>Count: {{ counter.count }} (doubled: {{ counter.double }})</p>
    <button @click="counter.increment">Increment</button>
    <button @click="counter.reset">Reset</button>
  </section>
</template>

<style scoped>
button {
  margin-right: 0.5rem;
  padding: 0.5rem 1rem;
  font-size: 1rem;
  cursor: pointer;
}
</style>
