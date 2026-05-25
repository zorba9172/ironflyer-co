<script setup lang="ts">
import { RouterLink, RouterView } from "vue-router";
</script>

<template>
  <header class="app-header">
    <div class="brand">Ironflyer Vue SPA</div>
    <nav>
      <RouterLink to="/">Home</RouterLink>
      <RouterLink to="/about">About</RouterLink>
    </nav>
  </header>
  <main class="app-main">
    <RouterView />
  </main>
</template>

<style>
:root {
  font-family: system-ui, sans-serif;
  color-scheme: light dark;
}
body {
  margin: 0;
}
.app-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 2rem;
  border-bottom: 1px solid rgba(127, 127, 127, 0.2);
}
.app-header nav a {
  margin-left: 1rem;
  text-decoration: none;
  color: inherit;
}
.app-header nav a.router-link-active {
  font-weight: 600;
}
.app-main {
  padding: 2rem;
  max-width: 960px;
  margin: 0 auto;
}
</style>
