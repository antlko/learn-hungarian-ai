<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { PenLine, Plus } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import EmptyState from '@/components/common/EmptyState.vue'
import Note from '@/components/common/Note.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import NewPieceDialog from '@/components/writing/NewPieceDialog.vue'
import { writingApi } from '@/api'
import { describeError } from '@/lib/errors'
import { shortDate } from '@/lib/writing'
import { useLevelStore } from '@/stores/level'
import { useUiStore } from '@/stores/ui'
import type { WritingListItem } from '@/types'

/** «Письмо» (docs/UX.md §7.2): the learner's pieces, newest edit first. */
const { t } = useI18n()
const ui = useUiStore()
const levelStore = useLevelStore()

const pieces = ref<WritingListItem[] | null>(null)
const error = ref('')
const creating = ref(false)

onMounted(async () => {
  levelStore.refresh()
  try {
    pieces.value = await writingApi.list()
  } catch (e) {
    error.value = describeError(e)
  }
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <PageHeader :title="t('writing.title')">
      <template #actions>
        <Button @click="creating = true"><Plus class="size-4" aria-hidden="true" />{{ t('writing.new') }}</Button>
      </template>
    </PageHeader>

    <Note v-if="error" variant="error">{{ error }}</Note>
    <div v-else-if="!pieces" class="flex flex-col gap-3">
      <Skeleton class="h-20 w-full" />
      <Skeleton class="h-20 w-full" />
    </div>
    <EmptyState v-else-if="!pieces.length" :icon="PenLine" :title="t('writing.title')" :body="t('writing.intro')">
      <Button @click="creating = true"><Plus class="size-4" aria-hidden="true" />{{ t('writing.new') }}</Button>
    </EmptyState>
    <ul v-else class="flex flex-col gap-3">
      <li v-for="p in pieces" :key="p.id">
        <RouterLink :to="{ name: 'write-piece', params: { id: p.id } }" class="block rounded-xl focus-visible:outline-2 focus-visible:outline-ring">
          <Card variant="quiet" interactive class="flex flex-col gap-1 p-4">
            <div class="flex flex-wrap items-baseline justify-between gap-x-3">
              <span class="font-medium">{{ p.title || t('writing.untitled') }}</span>
              <span class="text-xs text-muted-foreground tabular-nums">
                {{ p.level }} · {{ t('writing.words', { n: p.words }) }}
              </span>
            </div>
            <span class="text-xs text-muted-foreground">
              {{
                p.lastCheck
                  ? t('writing.checkedBy', { model: p.lastCheck.model, date: shortDate(p.lastCheck.createdAt, ui.locale) })
                  : t('writing.notChecked')
              }}
            </span>
          </Card>
        </RouterLink>
      </li>
    </ul>

    <NewPieceDialog :open="creating" @close="creating = false" />
  </div>
</template>
