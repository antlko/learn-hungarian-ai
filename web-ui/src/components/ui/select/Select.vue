<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{ class?: string; id?: string }>()
const model = defineModel<string>()
</script>

<template>
  <select
    :id="id"
    v-model="model"
    :class="
      cn(
        'flex h-9 w-full cursor-pointer rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50',
        $props.class,
      )
    "
  >
    <slot />
  </select>
</template>
