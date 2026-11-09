<script setup lang="ts">
/**
 * The one way to explain something without printing it.
 *
 * A small grey «?» that opens a short text. The card is TELEPORTED to <body> and
 * positioned with `position: fixed` from the button's rect, so no ancestor's
 * overflow or stacking context can clip it (it used to run off the unit panel's
 * edge). Placement: below the button and left-aligned to it, clamped so it always
 * stays inside the viewport with a 12px margin, flipped above when there is no
 * room below. On a phone it is a card above the tab bar instead. Re-measured on
 * open and on resize; any scroll closes it. Esc and an outside click close it and
 * focus returns to the button. Keep the slot to a sentence or two.
 *
 *   <InfoTip>Оценка по словарю и грамматике…</InfoTip>
 *   <InfoTip :title="$t('…')">…</InfoTip>
 */
import { nextTick, onUnmounted, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { CircleQuestionMark, X } from '@lucide/vue'
import { useDismiss } from '@/composables/useDismiss'

defineProps<{ title?: string; label?: string }>()

const { t } = useI18n()
const open = ref(false)
const id = useId()
const btn = ref<HTMLButtonElement | null>(null)
const card = ref<HTMLElement | null>(null)
const style = ref<Record<string, string>>({})
const MARGIN = 12

// Outside click and Escape close it; the `[data-infotip]` scope (on the button
// AND the teleported card) keeps a click on either from closing it.
useDismiss(open, `[data-infotip="${id}"]`)

function place() {
  const b = btn.value
  if (!b) return
  const vw = window.innerWidth
  const vh = window.innerHeight
  if (vw < 768) {
    // Phone: above the tab bar. The shell defines --bottom-tabs-h on an ancestor
    // of the button, which the teleported card no longer has, so read it here.
    const tabs = getComputedStyle(b).getPropertyValue('--bottom-tabs-h').trim() || '0px'
    style.value = { bottom: `calc(${tabs} + 1rem)` }
    return
  }
  const r = b.getBoundingClientRect()
  const width = Math.min(320, vw - 2 * MARGIN)
  const left = Math.min(Math.max(r.left, MARGIN), vw - MARGIN - width)
  const h = card.value?.offsetHeight ?? 0
  const below = vh - r.bottom - MARGIN - 4
  const above = r.top - MARGIN - 4
  if (h > below && above > below) {
    style.value = { left: `${left}px`, width: `${width}px`, bottom: `${vh - r.top + 4}px`, maxHeight: `${above}px` }
  } else {
    style.value = { left: `${left}px`, width: `${width}px`, top: `${r.bottom + 4}px`, maxHeight: `${Math.max(below, 120)}px` }
  }
}

function onScroll(e: Event) {
  if (card.value && e.target instanceof Node && card.value.contains(e.target)) return
  open.value = false
}

watch(open, async (o) => {
  window.removeEventListener('resize', place)
  window.removeEventListener('scroll', onScroll, true)
  if (o) {
    place()
    await nextTick()
    place() // second pass, now the card's height is known
    window.addEventListener('resize', place)
    window.addEventListener('scroll', onScroll, true)
  } else {
    btn.value?.focus({ preventScroll: true })
  }
})
onUnmounted(() => {
  window.removeEventListener('resize', place)
  window.removeEventListener('scroll', onScroll, true)
})
</script>

<template>
  <span class="relative inline-flex align-middle" :data-infotip="id">
    <button
      ref="btn"
      type="button"
      class="inline-flex size-7 cursor-pointer items-center justify-center rounded-full text-muted-foreground/70 transition-colors hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring"
      :aria-label="label ?? t('common.help')"
      :aria-expanded="open"
      :aria-controls="open ? id : undefined"
      @click.stop="open = !open"
    >
      <CircleQuestionMark class="size-4" aria-hidden="true" />
    </button>
    <Teleport to="body">
      <div
        v-if="open"
        :id="id"
        ref="card"
        :data-infotip="id"
        role="dialog"
        :aria-label="title ?? label ?? t('common.help')"
        :style="style"
        class="fixed inset-x-3 z-[100] overflow-y-auto rounded-xl border bg-popover p-4 text-sm leading-relaxed text-popover-foreground shadow-lg max-md:max-h-[60dvh] md:inset-x-auto md:bottom-auto"
      >
        <button
          type="button"
          class="absolute right-2 top-2 inline-flex size-7 cursor-pointer items-center justify-center rounded-full text-muted-foreground hover:text-foreground md:hidden"
          :aria-label="t('common.close')"
          @click="open = false"
        >
          <X class="size-4" aria-hidden="true" />
        </button>
        <p v-if="title" class="mb-1 pr-8 font-medium text-foreground md:pr-0">{{ title }}</p>
        <div class="pr-6 text-muted-foreground md:pr-0"><slot /></div>
      </div>
    </Teleport>
  </span>
</template>
