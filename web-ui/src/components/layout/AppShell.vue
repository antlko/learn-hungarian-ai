<script setup lang="ts">
import AppSidebar from './AppSidebar.vue'
import AppTopbar from './AppTopbar.vue'
</script>

<template>
  <!--
    h-dvh, not min-h-dvh: the shell must be exactly viewport height so that
    <main> is the only scrolling element. With min-h-dvh the container grows with
    its content, `overflow-y-auto` below never engages, and the sidebar and
    topbar scroll away with the page.
  -->
  <div class="flex h-dvh overflow-hidden">
    <AppSidebar />
    <div class="flex min-w-0 flex-1 flex-col">
      <AppTopbar />
      <main class="flex-1 overflow-y-auto p-6"><slot /></main>
    </div>
  </div>
</template>
