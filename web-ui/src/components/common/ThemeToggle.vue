<script setup lang="ts">
import { Moon, Sun } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()
</script>

<template>
  <Button
    variant="ghost"
    size="icon"
    :aria-label="$t('theme.toggle')"
    :title="$t('theme.toggle')"
    @click="ui.toggleTheme()"
  >
    <Moon v-if="ui.theme === 'dark'" class="size-4" />
    <Sun v-else class="size-4" />
  </Button>
</template>
