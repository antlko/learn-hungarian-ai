<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Toaster } from 'vue-sonner'
import AppShell from '@/components/layout/AppShell.vue'
import { useAuthStore } from '@/stores/auth'
import { useUiStore } from '@/stores/ui'

const route = useRoute()
const auth = useAuthStore()
const ui = useUiStore()

// Setup and login own the whole viewport: there is nothing to navigate to yet.
const chromeless = computed(() => route.meta.public === true || !auth.isAuthenticated)
</script>

<template>
  <Toaster :theme="ui.theme" position="bottom-right" rich-colors />

  <div v-if="!auth.ready" class="grid min-h-dvh place-items-center text-muted-foreground">
    <span class="text-sm">{{ $t('common.loading') }}</span>
  </div>

  <RouterView v-else-if="chromeless" />

  <AppShell v-else>
    <RouterView />
  </AppShell>
</template>

<style>
@import 'vue-sonner/style.css';
</style>
