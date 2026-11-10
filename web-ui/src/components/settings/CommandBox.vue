<script setup lang="ts">
import { onUnmounted, ref } from 'vue'
import { Check, Copy } from '@lucide/vue'
import { Button } from '@/components/ui/button'

/** A command the admin runs on the server, with a copy button that says it worked. */
const props = defineProps<{ command: string }>()

const copied = ref(false)
const failed = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

async function copy() {
  failed.value = false
  try {
    await navigator.clipboard.writeText(props.command)
    copied.value = true
  } catch {
    // No clipboard (plain http, old browser): the text is selectable, say so.
    failed.value = true
  }
  clearTimeout(timer)
  timer = setTimeout(() => {
    copied.value = false
    failed.value = false
  }, 2500)
}
onUnmounted(() => clearTimeout(timer))
</script>

<template>
  <div class="flex flex-col gap-1">
    <div class="flex flex-col items-stretch gap-2 sm:flex-row">
      <code
        class="min-w-0 flex-1 overflow-x-auto rounded bg-muted px-2 py-1.5 font-mono text-[11px] whitespace-pre"
        >{{ command }}</code
      >
      <Button size="sm" variant="outline" type="button" class="self-start sm:self-stretch" @click="copy">
        <Check v-if="copied" class="size-3.5" aria-hidden="true" />
        <Copy v-else class="size-3.5" aria-hidden="true" />
        {{ $t('settings.components.copy') }}
      </Button>
    </div>
    <p class="min-h-4 text-[11px] text-muted-foreground" role="status" aria-live="polite">
      <template v-if="copied">{{ $t('settings.components.copied') }}</template>
      <template v-else-if="failed">{{ $t('settings.components.copyFailed') }}</template>
    </p>
  </div>
</template>
