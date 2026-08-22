<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Check, EyeOff, Plus } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import Note from '@/components/common/Note.vue'
import { api } from '@/api'
import { useUiStore } from '@/stores/ui'
import type { LemmaDetail, LemmaStatus, StoredToken } from '@/types'

/**
 * The word detail rail.
 *
 * The important part is the breakdown of THIS form, not the dictionary entry.
 * Seeing that `házaimban` is `ház` + inessive + plural-possessum + first-person
 * possessor is the thing that makes the morphology legible; a gloss alone would
 * leave the learner with a word they cannot decompose next time.
 */
const props = defineProps<{ token: StoredToken | null; status: LemmaStatus | undefined }>()
const emit = defineEmits<{
  (e: 'statusChange', lemmaId: number, status: LemmaStatus): void
  (e: 'addCard', token: StoredToken): void
}>()

const ui = useUiStore()
const lemma = ref<LemmaDetail | null>(null)
const loading = ref(false)

watch(
  () => props.token?.lemmaId,
  async (id) => {
    lemma.value = null
    if (!id) return
    loading.value = true
    try {
      lemma.value = await api.lemma(id)
    } catch {
      lemma.value = null
    } finally {
      loading.value = false
    }
  },
  { immediate: true },
)

const glosses = computed(() => {
  const g = lemma.value?.glosses
  if (!g) return []
  const want = ui.locale === 'ru' ? 'ru' : 'en'
  return g[want]?.length ? g[want] : (g.en ?? g.ru ?? [])
})

const paradigmRows = computed(() => {
  const p = lemma.value?.paradigm
  if (!p) return []
  return Object.entries(p).sort(([a], [b]) => a.localeCompare(b))
})

function setStatus(status: LemmaStatus) {
  if (props.token?.lemmaId) emit('statusChange', props.token.lemmaId, status)
}
</script>

<template>
  <aside
    class="flex w-full shrink-0 flex-col gap-4 overflow-y-auto rounded-xl border border-border-subtle bg-card p-5 lg:w-80"
  >
    <p v-if="!token" class="text-sm text-muted-foreground">{{ $t('reader.pickAWord') }}</p>

    <template v-else>
      <div class="flex flex-col gap-1">
        <p class="font-mono text-xl" lang="hu">{{ token.form }}</p>
        <p v-if="lemma" class="text-sm text-muted-foreground">
          <span lang="hu">{{ lemma.lemma }}</span>
          <span class="ml-2 font-mono text-xs opacity-70">{{ lemma.upos }}</span>
          <span v-if="lemma.ipa" class="ml-2 font-mono text-xs opacity-70">{{ lemma.ipa }}</span>
        </p>
        <!-- A word the pack does not carry is a gap in the pack, not a fault:
             the same calm treatment as every other stated limitation. -->
        <Note v-else-if="!loading" variant="limit" bare class="text-xs">
          {{ $t('reader.noAnalysis') }}
        </Note>
      </div>

      <!-- The breakdown of THIS occurrence. -->
      <div v-if="token.feats?.length" class="flex flex-col gap-1.5">
        <p class="text-xs text-muted-foreground">{{ $t('reader.thisForm') }}</p>
        <div class="flex flex-wrap gap-1">
          <span
            v-for="f in token.feats"
            :key="f"
            class="rounded border border-border-subtle bg-background px-1.5 py-0.5 font-mono text-[11px]"
          >
            {{ f }}
          </span>
        </div>
      </div>

      <!-- An analysis we are not sure of says so. [ADR-002] -->
      <Note
        v-if="token.certainty === 'uncertain' || token.certainty === 'derived'"
        variant="limit"
        bare
        class="text-[11px]"
      >
        {{ $t(`reader.certainty.${token.certainty}`) }}
      </Note>

      <div v-if="glosses.length" class="flex flex-col gap-1">
        <p class="text-xs text-muted-foreground">
          {{ $t('reader.meaning') }}
          <span v-if="lemma?.glossFallback" class="text-uncertain">
            {{ $t('review.glossFallback') }}
          </span>
        </p>
        <ol class="list-inside list-decimal text-sm leading-relaxed">
          <li v-for="(g, i) in glosses.slice(0, 4)" :key="i">{{ g }}</li>
        </ol>
      </div>

      <p v-if="lemma?.freqRank" class="text-xs text-muted-foreground">
        {{ $t('reader.frequency', { rank: lemma.freqRank }) }}
        <span v-if="lemma.cefr" class="ml-1 font-mono">{{ lemma.cefr }}</span>
      </p>

      <div class="flex flex-wrap gap-1.5">
        <Button
          size="sm"
          :variant="status === 'known' ? 'secondary' : 'outline'"
          @click="setStatus('known')"
        >
          <Check class="size-3.5" />{{ $t('reader.markKnown') }}
        </Button>
        <Button
          size="sm"
          :variant="status === 'ignored' ? 'secondary' : 'ghost'"
          @click="setStatus('ignored')"
        >
          <EyeOff class="size-3.5" />{{ $t('reader.ignore') }}
        </Button>
        <Button size="sm" variant="default" @click="emit('addCard', token)">
          <Plus class="size-3.5" />{{ $t('reader.addCard') }}
        </Button>
      </div>

      <details v-if="paradigmRows.length" class="text-sm">
        <summary class="cursor-pointer text-xs text-muted-foreground">
          {{ $t('reader.paradigm') }}
        </summary>
        <table class="mt-2 w-full font-mono text-[11px]">
          <tbody>
            <tr v-for="[slot, form] in paradigmRows" :key="slot" class="border-b border-border-subtle">
              <td class="py-0.5 pr-2 text-muted-foreground">{{ slot }}</td>
              <td class="py-0.5" lang="hu">{{ form }}</td>
            </tr>
          </tbody>
        </table>
      </details>
    </template>
  </aside>
</template>
