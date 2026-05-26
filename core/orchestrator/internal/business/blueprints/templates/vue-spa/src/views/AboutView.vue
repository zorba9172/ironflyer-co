<script setup lang="ts">
const buildTime = new Date().toISOString();
</script>

<template>
  <section>
    <h1>About</h1>
    <p>
      This is the Ironflyer Vue SPA blueprint — a Vite-powered single
      page app with TypeScript, Vue Router, and Pinia ready out of the
      box. Reach for it when the spec calls for a snappy interactive UI
      without a Next.js server runtime.
    </p>
    <p>Build started at: {{ buildTime }}</p>
  </section>
</template>
