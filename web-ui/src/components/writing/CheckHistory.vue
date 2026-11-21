<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Trash2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { shortDate } from '@/lib/writing'
import { useUiStore } from '@/stores/ui'
import type { WritingCheckSummary } from '@/types'

/** The piece's checks, newest first. Each opens in full and can be deleted. */
defineProps<{ checks: WritingCheckSummary[]; shownId: number | null; busy?: boolean }>()
const emit = defineEmits<{ open: [id: number]; remove: [id: number] }>()
const { t } = useI18n()
const ui = useUiStore()
const asking = ref<number | null>(null)
</script>

<template>
  <section v-if="checks.length" class="flex flex-col gap-2">
    <h3 class="text-sm font-medium">{{ t('writing.history.title') }}</h3>
    <ul class="flex flex-col gap-2">
      <li
        v-for="c in checks"
        :key="c.id"
        class="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 rounded-lg border border-border-subtle bg-card px-3 py-2 text-sm"
        :class="shownId === c.id ? 'border-primary' : ''"
      >
        <div class="flex min-w-0 flex-col">
          <span class="truncate font-medium">{{ c.model }}</span>
          <span class="text-xs text-muted-foreground">
            {{ shortDate(c.createdAt, ui.locale) }} · {{ c.source === 'local' ? t('writing.history.local') : t('writing.history.external') }}
            <template v-if="c.status === 'failed'"> · {{ t('writing.history.unreadable') }}</template>
            <template v-else> · {{ t('writing.history.errors', { n: c.errorCount }) }}</template>
            <template v-if="c.status === 'partial'"> · {{ t('writing.history.partial') }}</template>
            <template v-if="c.interrupted"> · {{ t('writing.history.interrupted') }}</template>
            <template v-if="c.level"> · {{ c.level }}</template>
          </span>
        </div>
        <div class="flex items-center gap-1">
          <span v-if="shownId === c.id" class="text-xs text-muted-foreground">{{ t('writing.history.shown') }}</span>
          <Button v-else size="sm" variant="outline" class="max-md:min-h-11" :disabled="busy" @click="emit('open', c.id)">{{ t('writing.history.open') }}</Button>
          <Button
            v-if="asking !== c.id"
            size="icon"
            variant="ghost"
            class="max-md:size-11"
            :aria-label="t('writing.history.delete')"
            :title="t('writing.history.delete')"
            @click="asking = c.id"
          >
            <Trash2 class="size-4" aria-hidden="true" />
          </Button>
          <template v-else>
            <Button size="sm" variant="destructive" class="max-md:min-h-11" @click="emit('remove', c.id); asking = null">{{ t('writing.deleteYes') }}</Button>
            <Button size="sm" variant="ghost" class="max-md:min-h-11" @click="asking = null">{{ t('common.cancel') }}</Button>
          </template>
        </div>
      </li>
    </ul>
  </section>
</template>
