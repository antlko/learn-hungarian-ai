<script setup lang="ts">
import InfoTip from '@/components/common/InfoTip.vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Download, RefreshCw } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { useUiStore } from '@/stores/ui'
import type { MTStatus } from '@/types'

/**
 * The translator's own status, in detail — moved here from the Services card
 * (ADR-015). The generic probe flattens «качает модель» and «никогда не
 * разворачивали» into one grey «нет связи»; this block passes the service's
 * own words through. State and polling stay in SettingsView, which the
 * Services row also reads, so the two cannot disagree.
 */
const props = defineProps<{ mt: MTStatus | null; warming: boolean; isAdmin: boolean }>()
const emit = defineEmits<{ warmup: [] }>()

useI18n()
const ui = useUiStore()

/** Real progress or none: 0 total bytes means unmeasurable, never «0%». */
const progress = computed(() => {
  const m = props.mt
  if (!m?.totalBytes) return null
  return Math.min(100, Math.round(((m.downloadedBytes ?? 0) / m.totalBytes) * 100))
})

const gb = (bytes: number) =>
  (bytes / 1e9).toLocaleString(ui.locale === 'ru' ? 'ru-RU' : 'en-GB', {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
  })
</script>

<template>
  <div class="flex flex-col gap-2">
    <template v-if="!mt?.reachable">
      <p class="text-xs text-muted-foreground">{{ $t('settings.mt.notRunning') }}</p>
    </template>

    <template v-else-if="mt.state === 'loading'">
      <p class="text-xs text-learning">{{ $t('settings.mt.downloading') }}</p>
      <div class="h-1.5 w-full overflow-hidden rounded-full bg-muted">
        <div
          v-if="progress !== null"
          class="h-full rounded-full bg-learning transition-[width] duration-500"
          :style="{ width: `${progress}%` }"
        />
        <div v-else class="h-full w-1/3 animate-pulse rounded-full bg-learning" />
      </div>
      <p class="text-[11px] tabular-nums text-muted-foreground" role="status" aria-live="polite">
        <template v-if="progress !== null && mt.totalBytes">
          {{
            $t('settings.mt.progress', {
              done: gb(mt.downloadedBytes ?? 0),
              total: gb(mt.totalBytes),
              pct: progress,
            })
          }}
        </template>
        <template v-else>{{ $t('settings.mt.downloadingSize') }}</template>
      </p>
    </template>

    <template v-else-if="mt.state === 'failed'">
      <p class="text-xs text-error">{{ $t('settings.mt.failed') }}</p>
      <code
        v-if="mt.detail"
        class="max-h-24 overflow-auto rounded bg-muted px-2 py-1 font-mono text-[11px]"
        >{{ mt.detail }}</code
      >
    </template>

    <p v-else class="text-xs text-known">{{ $t('settings.mt.ready') }}</p>

    <Button
      v-if="mt?.reachable && mt.state !== 'ready'"
      size="sm"
      variant="outline"
      class="self-start"
      :disabled="warming || !isAdmin || mt.state === 'loading'"
      :title="isAdmin ? '' : $t('settings.mt.adminOnly')"
      @click="emit('warmup')"
    >
      <component
        :is="mt.state === 'failed' ? RefreshCw : Download"
        class="size-3.5"
        aria-hidden="true"
      />
      {{ mt.state === 'failed' ? $t('settings.mt.retry') : $t('settings.mt.download') }}
    </Button>

    <InfoTip class="self-start">{{ $t('settings.mt.hint') }}</InfoTip>
  </div>
</template>
