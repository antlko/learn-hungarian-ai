<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  BookOpen,
  Dumbbell,
  ExternalLink,
  Headphones,
  Library,
  Newspaper,
  Video,
  Wallet,
} from '@lucide/vue'
import Note from '@/components/common/Note.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { api } from '@/api'
import { useUiStore } from '@/stores/ui'
import type { Resource, ResourceKind } from '@/types'

/**
 * Where the course hands over.
 *
 * This is not a "more content" page. [R11]'s reading volume is about material
 * the learner chose, and the whole point of the syllabus ending is that they
 * start choosing — so this screen is a map of real Hungarian sources, levelled,
 * not a second course. See docs/PEDAGOGY.md §2.8.
 *
 * The catalogue ships in the language pack rather than in this file, so a dead
 * site can be fixed by a pack release instead of an app release.
 */
const ui = useUiStore()

/** Decorative tint per group, cycling; the group's heading is the label. */
const CHIP = [
  'border-tint-mist-edge bg-tint-mist',
  'border-tint-sage-edge bg-tint-sage',
  'border-tint-cream-edge bg-tint-cream',
  'border-tint-blush-edge bg-tint-blush',
]
const CARD = [
  'border-tint-mist-edge',
  'border-tint-sage-edge',
  'border-tint-cream-edge',
  'border-tint-blush-edge',
]

const items = ref<Resource[]>([])
const loading = ref(true)
const error = ref('')

// Ordered by how a learner leaving the course actually reaches for them: news
// and graded reading first, then listening and video, with the reference and
// practice shelves last because they are looked up, not consumed.
const KIND_ORDER: ResourceKind[] = [
  'news',
  'reading',
  'listening',
  'video',
  'reference',
  'practice',
]

const ICONS: Record<ResourceKind, typeof Newspaper> = {
  news: Newspaper,
  reading: BookOpen,
  listening: Headphones,
  video: Video,
  reference: Library,
  practice: Dumbbell,
}

const groups = computed(() =>
  KIND_ORDER.map((kind) => ({ kind, items: items.value.filter((r) => r.kind === kind) })).filter(
    (g) => g.items.length > 0,
  ),
)

const blurb = (r: Resource) =>
  ui.locale === 'ru' ? r.blurbRu || r.blurbEn : r.blurbEn || r.blurbRu

onMounted(async () => {
  try {
    items.value = (await api.resources()).resources
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="page-narrow flex w-full min-w-0 flex-col gap-6">
    <div class="flex flex-col gap-2">
      <h1 class="page-title">{{ $t('resources.title') }}</h1>
      <p class="max-w-2xl text-sm text-muted-foreground">{{ $t('resources.intro') }}</p>
    </div>

    <p v-if="loading" class="text-sm text-muted-foreground">{{ $t('common.loading') }}</p>
    <Note v-else-if="error" variant="error">{{ error }}</Note>

    <!-- A pack without a catalogue says so. An empty screen would read as a bug. -->
    <EmptyState v-else-if="!items.length" art="chainBridge" :title="$t('resources.empty')" />

    <div v-else class="flex flex-col gap-7">
      <section v-for="(g, gi) in groups" :key="g.kind" class="flex flex-col gap-2.5">
        <h2
          class="flex items-center gap-2 text-xs font-semibold tracking-widest text-muted-foreground uppercase"
        >
          <span
            class="inline-flex size-7 items-center justify-center rounded-lg border text-primary"
            :class="CHIP[gi % CHIP.length]"
            aria-hidden="true"
          >
            <component :is="ICONS[g.kind]" class="size-4" />
          </span>
          {{ $t(`resources.kind.${g.kind}`) }}
        </h2>

        <ul class="stagger-in grid gap-2 sm:grid-cols-2">
          <li v-for="r in g.items" :key="r.url">
            <a
              :href="r.url"
              target="_blank"
              rel="noopener noreferrer"
              class="flex h-full flex-col gap-1.5 rounded-xl border bg-card p-4 transition-colors hover:border-primary/50"
              :class="CARD[gi % CARD.length]"
            >
              <span class="flex items-start justify-between gap-2">
                <span class="text-sm font-medium">{{ r.title }}</span>
                <ExternalLink
                  class="mt-0.5 size-3.5 shrink-0 text-muted-foreground"
                  aria-hidden="true"
                />
              </span>

              <span class="flex flex-wrap items-center gap-2 text-xs">
                <span class="rounded-full border border-tint-mist-edge bg-tint-mist px-2 py-0.5 font-mono text-foreground">
                  {{ $t('resources.usableFrom', { level: r.level }) }}
                </span>
                <!-- Only the paywall is marked: labelling "free" on most of the
                     list would be noise, and a surprise paywall is the one thing
                     a catalogue must not spring on anyone. -->
                <span
                  v-if="!r.free"
                  class="flex items-center gap-1 rounded-full border border-tint-cream-edge bg-tint-cream px-2 py-0.5 text-foreground"
                >
                  <Wallet class="size-3" aria-hidden="true" />
                  {{ $t('resources.paid') }}
                </span>
              </span>

              <span v-if="blurb(r)" class="text-xs leading-relaxed text-muted-foreground">
                {{ blurb(r) }}
              </span>
            </a>
          </li>
        </ul>
      </section>
    </div>
  </div>
</template>
