<script setup lang="ts">
import { Button } from '@/components/ui/button'
import { useUiStore } from '@/stores/ui'

const ui = useUiStore()
</script>

<template>
  <Button
    variant="ghost"
    size="sm"
    class="font-mono text-xs uppercase"
    :aria-label="$t('locale.toggle')"
    :title="$t('locale.toggle')"
    @click="ui.setAppLocale(ui.locale === 'ru' ? 'en' : 'ru')"
  >
    {{ ui.locale }}
  </Button>
</template>
