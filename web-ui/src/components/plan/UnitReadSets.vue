<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Sparkles } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { api } from '@/api'
import { describeError } from '@/lib/errors'
import { useUiStore } from '@/stores/ui'
import type { LibrarySet } from '@/types'

/**
 * «Читать по теме»: the reading sets that belong to this unit (the pack's
 * «Фразы» under the unit's code, the AI-written «Диалог» under `<code>-dialogue`).
 * Optional: hidden when there are none or the fetch fails. Adding is an explicit
 * click and makes a text, never cards (ADR-066).
 */
const props = defineProps<{ unitCode: string }>()
const router = useRouter()
const ui = useUiStore()

const sets = ref<LibrarySet[]>([])
const importing = ref('')
const error = ref('')

const rows = computed(() =>
  sets.value
    .filter((s) => s.code === props.unitCode || s.code === `${props.unitCode}-dialogue`)
    .sort((a, b) => Number(a.authored) - Number(b.authored)),
)

function title(s: LibrarySet) {
  const loc = ui.locale === 'en' ? s.titleEn : s.titleRu
  return loc || s.title
}

async function load(code: string) {
  try {
    const r = await api.librarySets()
    if (code === props.unitCode) sets.value = Array.isArray(r) ? r : []
  } catch {
    if (code === props.unitCode) sets.value = []
  }
}

async function open(s: LibrarySet) {
  error.value = ''
  if (s.imported && s.textId) {
    router.push(`/read/${s.textId}`)
    return
  }
  importing.value = s.code
  try {
    const res = await api.importLibrarySet(s.code)
    router.push(`/read/${res.textId}`)
  } catch (e) {
    error.value = describeError(e)
  } finally {
    importing.value = ''
  }
}

watch(
  () => props.unitCode,
  (c) => {
    sets.value = []
    error.value = ''
    void load(c)
  },
  { immediate: true },
)
</script>

<template>
  <section v-if="rows.length" class="flex flex-col gap-2">
    <h2 class="text-xs font-semibold tracking-widest text-muted-foreground">{{ $t('plan.readTitle') }}</h2>
    <ul class="grid gap-2 sm:grid-cols-2">
      <li v-for="s in rows" :key="s.code" class="flex items-center gap-3 rounded-xl border bg-card px-3 py-2">
        <div class="min-w-0 flex-1">
          <p class="line-clamp-2 text-sm font-medium">{{ title(s) }}</p>
          <p class="text-xs text-muted-foreground">
            {{ s.authored ? $t('plan.readKindDialogue') : $t('plan.readKindPhrases') }} ·
            <span class="tabular-nums">{{ $t('library.sets.count', { n: s.sentences }) }}</span>
          </p>
          <p v-if="s.authored" class="mt-0.5 flex items-center gap-1 text-xs text-uncertain">
            <Sparkles class="size-3.5 shrink-0" aria-hidden="true" />
            {{ $t('library.sets.authored') }}
          </p>
        </div>
        <Button
          class="min-h-11 shrink-0"
          size="sm"
          :variant="s.imported ? 'outline' : 'default'"
          :disabled="importing === s.code"
          @click="open(s)"
        >
          {{ importing === s.code ? $t('common.saving') : s.imported ? $t('plan.readOpen') : $t('plan.readAdd') }}
        </Button>
      </li>
    </ul>
    <p v-if="error" role="alert" class="text-xs text-destructive">{{ error }}</p>
  </section>
</template>
