<script setup lang="ts">
import { ArrowLeft } from '@lucide/vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

/**
 * The way back from a sub-page. The app chrome has no browser back affordance on a
 * phone, so every sub-page (never the main tab screens) starts with one.
 *
 * It goes `router.back()` when the previous entry is where `fallback` points, so
 * the learner returns to the scroll position they left; otherwise (deep link,
 * reload) it navigates to `fallback`. A 44px target.
 */
const props = defineProps<{ fallback: string }>()
const { t } = useI18n()
const router = useRouter()

function go() {
  const prev = (router.options.history.state as { back?: string | null }).back
  const path = props.fallback
  if (prev && (prev === path || prev.startsWith(`${path}?`))) router.back()
  else void router.push(path)
}
</script>

<template>
  <button
    type="button"
    class="-ml-2 inline-flex min-h-11 cursor-pointer items-center gap-1.5 self-start rounded-lg px-2 text-sm text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
    @click="go"
  >
    <ArrowLeft class="size-4" aria-hidden="true" />
    {{ t('common.back') }}
  </button>
</template>
