<script setup lang="ts">
import type { Sidecar } from '@/lib/deploy'

/** Which model a sidecar downloads, under what licence, and where its code is. */
defineProps<{ sidecar: Sidecar }>()
</script>

<template>
  <i18n-t keypath="component.source.line" tag="p" class="text-[11px] text-muted-foreground">
    <template #model>
      <span class="break-all font-mono">{{ sidecar.model }}</span>
    </template>
    <template #licence>{{ sidecar.licence }}</template>
    <template #modelLink>
      <a :href="sidecar.modelUrl" target="_blank" rel="noopener noreferrer" class="underline underline-offset-2">{{ $t('component.source.modelPage') }}</a>
    </template>
    <template #sourceLink>
      <a :href="sidecar.sourceUrl" target="_blank" rel="noopener noreferrer" class="underline underline-offset-2">{{ $t('component.source.repo', { path: sidecar.sourceLabel }) }}</a>
    </template>
  </i18n-t>
</template>
