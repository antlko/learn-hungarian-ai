<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Check, Loader2, Sparkles } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import BottomSheet from '@/components/common/BottomSheet.vue'
import Note from '@/components/common/Note.vue'
import SpeakButton from '@/components/common/SpeakButton.vue'
import { Button } from '@/components/ui/button'
import { api } from '@/api'
import { describeError } from '@/lib/errors'
import { useUiStore } from '@/stores/ui'
import type { CourseProposal, CourseProposeResult } from '@/types'

/**
 * The learner's review of AI-written example sentences for unit words the pack
 * cannot illustrate (ADR-073).
 *
 * One request per word, in order, started when the word is reached; nothing is
 * stored until the learner taps «Добавить» on a sentence they have read. The
 * model's name is printed beside the sentences, in the open. Closing cancels:
 * a reply that arrives for a word the sheet has left is dropped.
 */
const props = defineProps<{
  open: boolean
  unitCode: string
  /** The words to go through (a snapshot: accepting does not shorten the walk). */
  lemmas: string[]
  /** lemma -> gloss, to say what the word means above its sentences. */
  glosses: Record<string, string>
}>()
const emit = defineEmits<{ close: []; changed: [] }>()

const { t } = useI18n()
const ui = useUiStore()

const index = ref(0)
const status = ref<'loading' | 'ready' | 'error'>('loading')
const errorText = ref('')
const result = ref<CourseProposeResult | null>(null)
const added = ref(new Set<string>())
const adding = ref('')
const addError = ref('')
const finished = ref(false)
/** Bumped on every request and on close, so a stale reply can tell. */
let ticket = 0

const lemma = computed(() => props.lemmas[index.value] ?? '')
const isLast = computed(() => index.value >= props.lemmas.length - 1)
const wordAdded = computed(() => !!result.value?.proposals.some((p) => added.value.has(p.hu)))
const stepLabel = computed(() =>
  isLast.value ? 'plan.aiDone' : wordAdded.value ? 'plan.aiNext' : 'plan.aiSkip',
)

async function load() {
  const my = ++ticket
  status.value = 'loading'
  errorText.value = ''
  addError.value = ''
  result.value = null
  try {
    const res = await api.proposeUnitWord(props.unitCode, lemma.value, ui.locale)
    if (my !== ticket) return
    result.value = res
    status.value = 'ready'
  } catch (e) {
    if (my !== ticket) return
    errorText.value = describeError(e)
    status.value = 'error'
  }
}

async function accept(p: CourseProposal) {
  if (adding.value) return
  adding.value = p.hu
  addError.value = ''
  try {
    await api.acceptUnitWord(props.unitCode, lemma.value, p)
    added.value = new Set(added.value).add(p.hu)
    emit('changed')
  } catch (e) {
    addError.value = describeError(e)
  } finally {
    adding.value = ''
  }
}

function next() {
  if (isLast.value) {
    ticket++
    finished.value = true
    return
  }
  index.value++
  void load()
}

function close() {
  ticket++
  emit('close')
}

watch(
  () => props.open,
  (o) => {
    if (!o) return
    index.value = 0
    added.value = new Set()
    finished.value = false
    if (props.lemmas.length) void load()
  },
)
</script>

<template>
  <BottomSheet :open="open" :title="t('plan.aiTitle')" dialog @close="close">
    <div class="flex flex-col gap-4 pb-2">
      <template v-if="finished">
        <Note variant="info">{{ t('plan.aiAllDone') }}</Note>
        <Button class="min-h-11 self-start" @click="close">{{ t('plan.aiClose') }}</Button>
      </template>

      <template v-else>
        <div class="flex items-baseline justify-between gap-3">
          <p class="min-w-0 text-base">
            <span class="font-semibold" lang="hu">{{ lemma }}</span>
            <span v-if="glosses[lemma]" class="text-muted-foreground"> – {{ glosses[lemma] }}</span>
          </p>
          <span v-if="lemmas.length > 1" class="shrink-0 font-mono text-xs tabular-nums text-muted-foreground">
            {{ t('plan.aiProgress', { n: index + 1, of: lemmas.length }) }}
          </span>
        </div>

        <p v-if="status === 'loading'" class="flex items-center gap-2 text-sm text-muted-foreground" role="status">
          <Loader2 class="size-4 animate-spin" aria-hidden="true" />
          {{ t('plan.aiLoading') }}
        </p>

        <template v-else-if="status === 'error'">
          <Note variant="error">{{ errorText }}</Note>
          <Button variant="outline" class="min-h-11 self-start" @click="load">{{ t('plan.aiRetry') }}</Button>
        </template>

        <template v-else-if="result">
          <p class="flex items-start gap-1.5 text-xs text-muted-foreground">
            <Sparkles class="mt-0.5 size-3.5 shrink-0 text-uncertain" aria-hidden="true" />
            {{ t('plan.aiModel', { model: result.model }) }}
          </p>
          <Note v-if="!result.proposals.length" variant="info">{{ t('plan.aiEmpty') }}</Note>
          <ul v-else class="flex flex-col divide-y divide-border-subtle rounded-xl border">
            <li v-for="p in result.proposals" :key="p.hu" class="flex items-center gap-2 px-3 py-2">
              <span class="flex min-w-0 flex-1 flex-col">
                <span class="font-medium" lang="hu">{{ p.hu }}</span>
                <span v-if="p.tr" class="text-sm text-muted-foreground">{{ p.tr }}</span>
              </span>
              <SpeakButton :text="p.hu" class="shrink-0" />
              <span v-if="added.has(p.hu)" class="inline-flex min-h-11 shrink-0 items-center gap-1 px-2 text-xs text-known">
                <Check class="size-4" aria-hidden="true" />
                {{ t('plan.aiAdded') }}
              </span>
              <Button
                v-else
                size="sm"
                variant="secondary"
                class="min-h-11 shrink-0"
                :disabled="!!adding"
                @click="accept(p)"
              >
                {{ adding === p.hu ? '…' : t('plan.aiAdd') }}
              </Button>
            </li>
          </ul>
          <Note v-if="addError" variant="error">{{ addError }}</Note>
        </template>

        <div class="flex flex-wrap items-center gap-2">
          <Button v-if="status !== 'loading'" class="min-h-11" @click="next">{{ t(stepLabel) }}</Button>
          <Button variant="ghost" class="min-h-11" @click="close">{{ t('plan.aiClose') }}</Button>
        </div>
      </template>
    </div>
  </BottomSheet>
</template>
