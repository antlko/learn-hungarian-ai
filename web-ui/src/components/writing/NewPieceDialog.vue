<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { PenLine } from '@lucide/vue'
import BottomSheet from '@/components/common/BottomSheet.vue'
import Note from '@/components/common/Note.vue'
import { Skeleton } from '@/components/ui/skeleton'
import { writingApi } from '@/api'
import { describeError } from '@/lib/errors'
import { useLevelStore } from '@/stores/level'
import { useUiStore } from '@/stores/ui'
import type { WritingLevel, WritingTopic } from '@/types'

/**
 * «Новое сочинение»: a level filter (the estimated level when there is one,
 * else A2), topic cards and a free topic. The advised length is advice only.
 */
const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const router = useRouter()
const ui = useUiStore()
const level = useLevelStore()

const LEVELS: WritingLevel[] = ['A1', 'A2', 'B1', 'B2']
const topics = ref<WritingTopic[] | null>(null)
const error = ref('')
const busy = ref(false)
const picked = ref<WritingLevel>('A2')
let chosen = false

function defaultLevel(): WritingLevel {
  const band = level.level?.band ?? ''
  return (LEVELS as string[]).includes(band) ? (band as WritingLevel) : 'A2'
}

watch(
  () => props.open,
  async (open) => {
    if (!open) return
    error.value = ''
    chosen = false
    picked.value = defaultLevel()
    void level.refresh()
    if (topics.value) return
    try {
      topics.value = await writingApi.topics()
    } catch (e) {
      error.value = describeError(e)
    }
  },
  { immediate: true },
)

// The estimate may arrive after the dialog opened; it only sets the default.
watch(
  () => level.level?.band,
  () => {
    if (!chosen) picked.value = defaultLevel()
  },
)

function choose(l: WritingLevel) {
  chosen = true
  picked.value = l
}

const shown = computed(() => (topics.value ?? []).filter((x) => x.level === picked.value))
const pick = (ru: string, en: string) => (ui.locale === 'ru' ? ru : en)

async function create(topic?: WritingTopic) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    const piece = await writingApi.create(
      topic
        ? { topicCode: topic.code, level: topic.level }
        : { level: picked.value, title: t('writing.freeTitle') },
    )
    emit('close')
    await router.push({ name: 'write-piece', params: { id: piece.id } })
  } catch (e) {
    error.value = describeError(e)
  } finally {
    busy.value = false
  }
}

const CARD =
  'flex w-full cursor-pointer flex-col gap-1 rounded-xl border border-border bg-card p-3 text-left transition-colors hover:bg-accent focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50'
</script>

<template>
  <BottomSheet dialog tall :open="open" :title="t('writing.newTitle')" @close="emit('close')">
    <div class="flex flex-col gap-3 pb-2">
      <Note v-if="error" variant="error">{{ error }}</Note>

      <div role="radiogroup" :aria-label="t('writing.levelFilter')" class="flex flex-wrap gap-2">
        <button
          v-for="l in LEVELS"
          :key="l"
          type="button"
          role="radio"
          :aria-checked="picked === l"
          class="min-h-9 min-w-12 cursor-pointer rounded-full border px-3 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-ring"
          :class="picked === l ? 'border-primary bg-primary text-primary-foreground' : 'border-border hover:bg-accent'"
          @click="choose(l)"
        >
          {{ l }}
        </button>
      </div>

      <button type="button" :class="CARD" :disabled="busy" @click="create()">
        <span class="flex items-center gap-2 font-medium"><PenLine class="size-4 text-primary" aria-hidden="true" />{{ t('writing.free') }}</span>
        <span class="text-muted-foreground">{{ t('writing.freeBody') }}</span>
      </button>

      <Skeleton v-if="!topics && !error" class="h-20 w-full" />
      <p v-else-if="topics && !shown.length" class="text-muted-foreground">{{ t('writing.noTopics') }}</p>
      <button v-for="x in shown" :key="x.code" type="button" :class="CARD" :disabled="busy" @click="create(x)">
        <span class="font-medium">{{ pick(x.titleRu, x.titleEn) }}</span>
        <span class="text-muted-foreground">{{ pick(x.briefRu, x.briefEn) }}</span>
        <span class="text-xs text-muted-foreground">
          {{ t(`writing.genre.${x.genre}`) }} · {{ t('writing.length', { min: x.minWords, max: x.maxWords }) }}
        </span>
      </button>
    </div>
  </BottomSheet>
</template>
