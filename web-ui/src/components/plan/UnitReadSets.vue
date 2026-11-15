<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Button } from '@/components/ui/button'
import { api } from '@/api'
import { describeError } from '@/lib/errors'
import { useI18n } from 'vue-i18n'
import { useUiStore } from '@/stores/ui'
import { isColloquial, isKnownGenre, tierOf, TIER_ORDER } from '@/lib/readingSets'
import type { LibrarySet } from '@/types'

/**
 * «Читать по теме»: the three reading texts of this unit — short
 * (`<code>-dialogue`), medium (`<code>`) and long (`<code>-long`), in that order.
 * Below it «Живая речь» (colloquial register, ADR-077): the unit's talk as a card,
 * then its mini-dialogues as a compact list (first three, the rest behind «Ещё N»).
 * Optional: hidden when there are none or the fetch fails. Adding is an explicit
 * click and makes a text, never cards (ADR-066).
 */
const props = defineProps<{ unitCode: string }>()
const router = useRouter()
const ui = useUiStore()
const { t } = useI18n()

const sets = ref<LibrarySet[]>([])
const importing = ref('')
const error = ref('')

const rows = computed(() =>
  sets.value
    .filter((s) => [props.unitCode, `${props.unitCode}-dialogue`, `${props.unitCode}-long`].includes(s.code))
    .sort((a, b) => TIER_ORDER.indexOf(tierOf(a)) - TIER_ORDER.indexOf(tierOf(b))),
)

const talk = computed(() => sets.value.find((s) => s.code === `${props.unitCode}-talk` && isColloquial(s)))
const minis = computed(() =>
  sets.value.filter((s) => tierOf(s) === 'mini' && isColloquial(s) && s.code.startsWith(`${props.unitCode}-talk-`)),
)
/** One look for every text card: a compact row on a phone, a column from sm up. */
const CARD = 'flex items-center gap-3 rounded-xl border bg-card px-3 py-2 sm:flex-col sm:items-stretch'
const META = 'mt-1 flex flex-wrap items-center gap-x-1.5 gap-y-1 text-xs text-muted-foreground'
const CHIP = 'rounded-full border px-2 py-0.5 font-medium text-foreground'
const MINIS_SHOWN = 3
const allMinis = ref(false)
const shownMinis = computed(() => (allMinis.value ? minis.value : minis.value.slice(0, MINIS_SHOWN)))
const hasTalkBlock = computed(() => !!talk.value || minis.value.length > 0)

/** The genre label, unless the title already starts with it ("Записка: …"). */
function genreLabel(s: LibrarySet): string {
  if (!isKnownGenre(s.genre)) return ''
  const label = t(`library.genre.${s.genre}`)
  return title(s).toLowerCase().startsWith(label.toLowerCase()) ? '' : label
}

/** A mini-dialogue is named by its one-line situation; the title is the fallback. */
function scene(s: LibrarySet) {
  const loc = ui.locale === 'en' ? s.sceneEn : s.sceneRu
  return loc || s.scene || title(s)
}

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
    allMinis.value = false
    void load(c)
  },
  { immediate: true },
)
</script>

<template>
  <div v-if="rows.length || hasTalkBlock" class="flex flex-col gap-4">
  <section v-if="rows.length" class="flex flex-col gap-2">
    <h2 class="text-xs font-semibold tracking-widest text-muted-foreground">{{ $t('plan.readTitle') }}</h2>
    <ul class="grid gap-2 sm:grid-cols-3">
      <li v-for="s in rows" :key="s.code" :class="CARD">
        <div class="min-w-0 flex-1">
          <p class="line-clamp-2 text-sm font-medium">{{ title(s) }}</p>
          <p :class="META">
            <span :class="CHIP">{{ $t(`library.sets.tier.${tierOf(s)}`) }}</span>
            <span v-if="s.words" class="tabular-nums">{{ $t('library.sets.words', { n: s.words }, s.words) }}</span>
            <span v-if="genreLabel(s)">· {{ genreLabel(s) }}</span>
          </p>
        </div>
        <Button
          class="min-h-11 shrink-0 sm:w-full"
          size="sm"
          :variant="s.imported ? 'outline' : 'default'"
          :disabled="importing === s.code"
          @click="open(s)"
        >
          {{ importing === s.code ? $t('common.saving') : s.imported ? $t('plan.readOpen') : $t('library.sets.addRead') }}
        </Button>
      </li>
    </ul>
  </section>

  <section v-if="hasTalkBlock" class="flex flex-col gap-2">
    <h2 class="text-xs font-semibold tracking-widest text-muted-foreground">{{ $t('plan.talkTitle') }}</h2>
    <div v-if="talk" class="flex items-center gap-3 rounded-xl border bg-card px-3 py-2">
      <div class="min-w-0 flex-1">
        <p class="line-clamp-2 text-sm font-medium">{{ title(talk) }}</p>
        <p :class="META">
          <span :class="CHIP">{{ $t('library.sets.colloquial') }}</span>
          <span v-if="talk.words" class="tabular-nums">{{ $t('library.sets.words', { n: talk.words }, talk.words) }}</span>
          <span v-if="genreLabel(talk)">· {{ genreLabel(talk) }}</span>
        </p>
      </div>
      <Button
        class="min-h-11 shrink-0 sm:w-auto"
        size="sm"
        :variant="talk.imported ? 'outline' : 'default'"
        :disabled="importing === talk.code"
        @click="open(talk)"
      >
        {{ importing === talk.code ? $t('common.saving') : talk.imported ? $t('plan.readOpen') : $t('library.sets.addRead') }}
      </Button>
    </div>
    <ul v-if="minis.length" class="flex flex-col divide-y rounded-xl border bg-card">
      <li v-for="s in shownMinis" :key="s.code" class="flex min-h-11 items-center gap-2 pl-3">
        <p class="line-clamp-2 min-w-0 flex-1 py-2 text-sm">{{ scene(s) }}</p>
        <span v-if="s.words" class="shrink-0 text-xs tabular-nums text-muted-foreground">
          {{ $t('library.sets.words', { n: s.words }, s.words) }}
        </span>
        <Button
          class="min-h-11 shrink-0"
          size="sm"
          variant="ghost"
          :disabled="importing === s.code"
          @click="open(s)"
        >
          {{ importing === s.code ? $t('common.saving') : $t('plan.readOpen') }}
        </Button>
      </li>
    </ul>
    <Button
      v-if="minis.length > MINIS_SHOWN"
      class="min-h-11 self-start"
      size="sm"
      variant="outline"
      :aria-expanded="allMinis"
      @click="allMinis = !allMinis"
    >
      {{ allMinis ? $t('plan.talkFewer') : $t('plan.talkMore', { n: minis.length - MINIS_SHOWN }) }}
    </Button>
  </section>
  <p v-if="error" role="alert" class="text-xs text-destructive">{{ error }}</p>
  </div>
</template>
