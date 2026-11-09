<script setup lang="ts">
import { computed, nextTick, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronDown, X } from '@lucide/vue'
import { Skeleton } from '@/components/ui/skeleton'
import { useDismiss } from '@/composables/useDismiss'
import { useLevelStore } from '@/stores/level'

/**
 * The estimated level, in the top bar (ADR-047).
 *
 * The chip shows ONLY the label («A1.1»), the same height and type as the due and
 * streak chips beside it. Everything else — coverage, confidence, per-band bars,
 * the method sentence — lives in the popover. A measurement with a stated method,
 * not a score: it can go down, the method is on the panel every time it opens,
 * never behind a tooltip. On a failed fetch the chip is absent rather than alarming.
 *
 * The popover follows InfoTip: teleported to <body>, `fixed`, clamped to the
 * viewport, Esc / outside click close it, focus returns to the chip. On a phone
 * it is a full-width card above the tab bar.
 */
const { t } = useI18n()
const store = useLevelStore()
const route = useRoute()
const open = ref(false)
const btn = ref<HTMLButtonElement | null>(null)
const card = ref<HTMLElement | null>(null)
const style = ref<Record<string, string>>({})
const MARGIN = 12
const WIDTH = 320 // w-80
useDismiss(open, '[data-level]')
watch(() => route.fullPath, () => (open.value = false))

const lv = computed(() => store.level)
const pct = computed(() => Math.round((lv.value?.coverage ?? 0) * 100))
const conf = computed(() => (lv.value ? t(`level.confidence.${lv.value.confidence}`) : ''))

function place() {
  const b = btn.value
  if (!b) return
  const vw = window.innerWidth
  const vh = window.innerHeight
  if (vw < 768) {
    // The shell defines --bottom-tabs-h on an ancestor of the chip, not of the teleported card.
    const tabs = getComputedStyle(b).getPropertyValue('--bottom-tabs-h').trim() || '0px'
    style.value = { bottom: `calc(${tabs} + 1rem)` }
    return
  }
  const r = b.getBoundingClientRect()
  const width = Math.min(WIDTH, vw - 2 * MARGIN)
  const left = Math.min(Math.max(r.right - width, MARGIN), vw - MARGIN - width)
  style.value = {
    left: `${left}px`,
    width: `${width}px`,
    top: `${r.bottom + 8}px`,
    maxHeight: `${Math.max(vh - r.bottom - 8 - MARGIN, 160)}px`,
  }
}
watch(open, async (o) => {
  window.removeEventListener('resize', place)
  if (o) {
    place()
    await nextTick()
    place()
    window.addEventListener('resize', place)
  } else {
    btn.value?.focus({ preventScroll: true })
  }
})
onUnmounted(() => window.removeEventListener('resize', place))
</script>

<template>
  <div v-if="store.loading && !lv" class="px-1" aria-hidden="true">
    <Skeleton class="h-9 w-20 rounded-full" />
  </div>
  <div v-else-if="lv" class="relative" data-level>
    <button
      ref="btn"
      type="button"
      class="flex h-9 cursor-pointer items-center gap-1 rounded-full border border-tint-mist-edge bg-tint-mist px-3 text-sm font-semibold tabular-nums transition-colors hover:bg-tint-mist/70 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
      :aria-expanded="open"
      aria-haspopup="dialog"
      :aria-label="$t('level.chipAria', { label: lv.label, conf })"
      @click="open = !open"
    >
      {{ lv.label }}
      <ChevronDown class="size-3.5 text-muted-foreground" aria-hidden="true" />
    </button>

    <Teleport to="body">
      <div
        v-if="open"
        ref="card"
        data-level
        role="dialog"
        :aria-label="$t('level.title')"
        :style="style"
        class="fixed inset-x-3 z-[100] flex flex-col gap-4 overflow-y-auto rounded-xl border bg-popover p-4 text-sm text-popover-foreground shadow-lg max-md:max-h-[70dvh] md:inset-x-auto"
      >
        <button
          type="button"
          class="absolute top-2 right-2 inline-flex size-9 cursor-pointer items-center justify-center rounded-full text-muted-foreground hover:text-foreground md:hidden"
          :aria-label="$t('common.close')"
          @click="open = false"
        >
          <X class="size-4" aria-hidden="true" />
        </button>

        <!-- The level, the one-line meaning, how sure we are. -->
        <section class="flex flex-col gap-2">
          <p class="text-xs text-muted-foreground">{{ $t('level.title') }}</p>
          <p class="text-3xl font-semibold tabular-nums">{{ lv.label }}</p>
          <p>{{ $t('level.coverage', { pct }) }}</p>
          <p>
            <span class="inline-flex rounded-full border border-tint-cream-edge bg-tint-cream px-2.5 py-0.5 text-xs font-medium text-foreground">
              {{ conf }}
            </span>
          </p>
        </section>

        <section class="flex flex-col gap-2 border-t border-border-subtle pt-3">
          <p class="text-xs font-medium text-muted-foreground">{{ $t('level.bands') }}</p>
          <ul class="flex flex-col gap-2.5" :aria-label="$t('level.bands')">
            <li v-for="b in lv.bands" :key="b.band" class="flex flex-col gap-1">
              <div class="flex items-baseline justify-between gap-2">
                <span class="font-medium">{{ b.band }}</span>
                <span class="text-xs text-muted-foreground tabular-nums">
                  {{ $t('level.bandCount', { counted: b.counted, total: b.total }) }}
                </span>
              </div>
              <div
                class="relative h-2 rounded-full bg-muted"
                role="meter"
                :aria-valuemin="0"
                :aria-valuemax="100"
                :aria-valuenow="Math.round(b.share * 100)"
                :aria-label="b.band"
              >
                <div
                  class="h-full rounded-full"
                  :class="b.reached ? 'bg-known' : 'bg-primary/70'"
                  :style="{ width: `${Math.min(100, b.share * 100)}%` }"
                />
                <span
                  class="absolute -top-0.5 h-3 w-px bg-foreground/60"
                  :style="{ left: `${b.target * 100}%` }"
                  aria-hidden="true"
                />
              </div>
            </li>
          </ul>
          <p class="text-xs text-muted-foreground">{{ $t('level.target') }}</p>
        </section>

        <p class="border-t border-border-subtle pt-3 text-xs leading-relaxed text-muted-foreground">
          {{ $t(lv.methodKey) }}
        </p>
        <RouterLink to="/progress" class="min-h-9 self-start font-medium text-primary hover:underline">
          {{ $t('level.more') }}
        </RouterLink>
      </div>
    </Teleport>
  </div>
</template>
