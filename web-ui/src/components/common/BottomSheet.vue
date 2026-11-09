<script setup lang="ts">
/**
 * A sheet that slides up from the bottom on a phone — and, with `dialog`, a
 * centred dialog from md up. Teleported to <body>, so no ancestor clips it.
 *
 * Behaviour: 200ms slide + fade (none under reduced motion), max-height 75dvh,
 * a grab handle, a visible ✕, a backdrop tap and Esc both close it. Focus
 * moves into the sheet when it opens and back to the opener when it closes, and
 * Tab wraps inside it (trap-lite). It sits above the tab bar (z-30) and keeps
 * the home-indicator inset; while the on-screen keyboard is up it is lifted by
 * the keyboard's height and capped to what is still visible.
 *
 * Used by the review Teacher (<md) and «Как это работает». An InfoTip inside
 * still works: it is z-100.
 */
import { nextTick, onUnmounted, ref, watch } from 'vue'
import { X } from '@lucide/vue'
import { useKeyboardInset } from '@/composables/useKeyboardInset'

const props = defineProps<{
  open: boolean
  /** Accessible name, and the visible title unless `hideTitle`. */
  title: string
  hideTitle?: boolean
  /** From md up, show a centred dialog instead of a bottom sheet. */
  dialog?: boolean
  /**
   * From md up, a drawer on the right edge below the topbar: no scrim, the page
   * stays usable, and a click outside does not close it. Below md it is the
   * bottom sheet as ever.
   */
  side?: 'right'
}>()
const emit = defineEmits<{ close: [] }>()

const { inset, height } = useKeyboardInset()

/** `rendered` mounts the nodes; `shown` flips the classes a frame later so the transition runs. */
const rendered = ref(false)
const shown = ref(false)
const panel = ref<HTMLElement | null>(null)
let opener: HTMLElement | null = null
let timer: number | undefined

const reduced = () =>
  typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

const isSide = () => props.side === 'right' && window.matchMedia?.('(min-width: 768px)').matches

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    // A control inside (the tutor composer's Esc = stop) may have used it.
    if (e.defaultPrevented) return
    e.preventDefault()
    emit('close')
    return
  }
  // A drawer does not trap Tab: the card beside it stays reachable.
  if (e.key !== 'Tab' || !panel.value || isSide()) return
  const nodes = [
    ...panel.value.querySelectorAll<HTMLElement>(
      'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  ].filter((n) => n.offsetParent !== null)
  if (!nodes.length) {
    e.preventDefault()
    panel.value.focus()
    return
  }
  const first = nodes[0]!
  const last = nodes[nodes.length - 1]!
  const active = document.activeElement
  if (e.shiftKey && (active === first || active === panel.value)) {
    e.preventDefault()
    last.focus()
  } else if (!e.shiftKey && active === last) {
    e.preventDefault()
    first.focus()
  }
}

watch(
  () => props.open,
  async (o) => {
    window.clearTimeout(timer)
    if (o) {
      opener = document.activeElement as HTMLElement | null
      rendered.value = true
      document.addEventListener('keydown', onKey)
      await nextTick()
      panel.value?.focus({ preventScroll: true })
      requestAnimationFrame(() => (shown.value = true))
    } else {
      document.removeEventListener('keydown', onKey)
      shown.value = false
      timer = window.setTimeout(() => (rendered.value = false), reduced() ? 0 : 200)
      opener?.focus?.({ preventScroll: true })
      opener = null
    }
  },
  { immediate: true },
)
onUnmounted(() => {
  document.removeEventListener('keydown', onKey)
  window.clearTimeout(timer)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="rendered"
      class="fixed inset-0 z-50 flex items-end"
      :class="[dialog ? 'md:items-center md:justify-center' : '', side ? 'md:pointer-events-none' : '']"
    >
      <div
        class="absolute inset-0 bg-foreground/40 transition-opacity duration-200 motion-reduce:transition-none"
        :class="[shown ? 'opacity-100' : 'opacity-0', side ? 'md:hidden' : '']"
        aria-hidden="true"
        @click="emit('close')"
      />
      <div
        ref="panel"
        role="dialog"
        :aria-modal="side ? undefined : true"
        :aria-label="title"
        tabindex="-1"
        :style="
          inset > 0
            ? { bottom: `${inset}px`, maxHeight: `min(75dvh, ${Math.max(height - 16, 200)}px)` }
            : undefined
        "
        class="absolute inset-x-0 bottom-0 flex max-h-[75dvh] w-full flex-col rounded-t-2xl border bg-background shadow-raised outline-none transition-[transform,opacity] duration-200 motion-reduce:transition-none"
        :class="[
          shown ? 'translate-y-0 opacity-100' : 'translate-y-full opacity-0',
          dialog ? 'md:static md:max-w-lg md:rounded-2xl' : '',
          dialog && !shown ? 'md:translate-y-2' : '',
          side
            ? 'md:pointer-events-auto md:fixed md:top-[calc(3.5rem+env(safe-area-inset-top))] md:right-0 md:bottom-0 md:left-auto md:max-h-none md:w-96 md:max-w-[90vw] md:rounded-l-2xl md:rounded-tr-none md:border-r-0'
            : '',
          side && shown ? 'md:translate-x-0 md:translate-y-0' : '',
          side && !shown ? 'md:translate-x-full md:translate-y-0' : '',
        ]"
      >
        <div class="mx-auto mt-2 h-1 w-10 shrink-0 rounded-full bg-border" aria-hidden="true" :class="dialog || side ? 'md:hidden' : ''" />
        <div class="flex shrink-0 items-center justify-between gap-3 px-5 pt-2 pb-1">
          <h2 class="font-display text-lg font-medium" :class="hideTitle ? 'sr-only' : ''">{{ title }}</h2>
          <button
            type="button"
            class="-mr-2 inline-flex size-11 shrink-0 cursor-pointer items-center justify-center rounded-lg text-muted-foreground hover:bg-accent hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
            :aria-label="$t('common.close')"
            :title="$t('common.close')"
            @click="emit('close')"
          >
            <X class="size-5" aria-hidden="true" />
          </button>
        </div>
        <div
          class="min-h-0 flex-1 overflow-y-auto overscroll-contain px-5 text-sm leading-relaxed"
          :class="inset > 0 ? 'pb-3' : 'pb-[calc(env(safe-area-inset-bottom)+1.25rem)]'"
        >
          <slot />
        </div>
      </div>
    </div>
  </Teleport>
</template>
