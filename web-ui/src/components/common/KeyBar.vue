<script lang="ts">
/** One row of the map: the caps to draw, and what they do. */
export interface KeyHint {
  /** The caps, in order. `Space`, `Enter`, `Backspace` and the arrows get glyphs. */
  keys: string[]
  label: string
  /** Present but not available right now — drawn dim, still readable. */
  muted?: boolean
}
</script>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { ChevronUp, X } from '@lucide/vue'
import { cn } from '@/lib/utils'

/**
 * The keyboard map, where the hands are.
 *
 * "Keyboard first, everywhere" (docs/UX.md §1.2) was true of the code and not
 * of the screen: the reader and the review card each ended in one grey line of
 * `text-xs` below the fold, which a learner reported as the feature not
 * existing. Sentence translation had been on `t` for a while and was never
 * found. A shortcut nobody can see is a shortcut nobody uses.
 *
 * So the map is a fixed strip at the bottom of the viewport with the keys drawn
 * as keys. It is deliberately NOT interactive:
 *
 *  - `role="group"` and no tab stops. It documents the keyboard; it is not a
 *    second way to press these keys, and adding six focus stops to a reading
 *    loop would cost more than the strip is worth.
 *  - the caps are `aria-hidden` and each item carries its own readable label,
 *    so a screen reader gets "next word" rather than "arrow right arrow left".
 *
 * Dismissible, because a permanent hint becomes furniture you stop seeing —
 * and remembered per device, beside the other device preferences.
 */
const props = withDefaults(defineProps<{ items: KeyHint[]; storageKey?: string }>(), {
  storageKey: 'tanulom.keybar',
})

/** The glyph a key is drawn as. Anything not here is drawn as its own name. */
const GLYPHS: Record<string, string> = {
  Space: '␣',
  Enter: '↵',
  Backspace: '⌫',
  Delete: '⌦',
  Escape: 'Esc',
  ArrowLeft: '←',
  ArrowRight: '→',
  ArrowUp: '↑',
  ArrowDown: '↓',
}

const open = ref(localStorage.getItem(props.storageKey) !== 'off')

function setOpen(v: boolean) {
  open.value = v
  localStorage.setItem(props.storageKey, v ? 'on' : 'off')
}

const cap = (k: string) => GLYPHS[k] ?? k

/** The space bar is drawn as a bar. `␣` in a 20px cap reads as "a small box",
 *  not as the widest key on the keyboard; the shape is the whole hint. */
const capWidth = (k: string) => (k === 'Space' ? 'min-w-10' : 'min-w-5')
const shown = computed(() => props.items.filter((i) => i.keys.length))
</script>

<template>
  <!--
    Fixed to the viewport rather than sticky inside <main>: `AppShell` makes
    <main> the only scrolling element, so a sticky child would ride the scroll
    container and slide under the topbar. The shell pays for this with bottom
    padding on <main> — without it the last line of a text sits behind the strip.
  -->
  <!--
    Wraps rather than scrolls. `overflow-x-auto` on a strip this size hides the
    last item behind the edge with nothing saying it is there — on a 664px
    viewport «перевести» read as «перевест» and looked like a rendering bug. A
    second row is honest and costs 20px.
  -->
  <div
    v-if="shown.length"
    class="pointer-events-none fixed bottom-[var(--bottom-tabs-h,0px)] left-(--rail) right-0 z-30 flex justify-center px-4 pb-3 max-md:hidden [@media(pointer:coarse)]:hidden"
  >
    <div
      v-if="open"
      role="group"
      :aria-label="$t('keys.title')"
      class="pointer-events-auto flex max-w-full items-center gap-2 rounded-xl border border-border-subtle bg-card/85 px-3 py-2 shadow-raised backdrop-blur-md"
    >
      <!-- Only the hints wrap. With the close button in the same wrapping flow
           it was the item that got pushed onto a second line, alone and
           centred, which read as a stray control rather than a dismiss. -->
      <span class="flex min-w-0 flex-wrap items-center justify-center gap-x-4 gap-y-1.5">
      <span
        v-for="(item, i) in shown"
        :key="i"
        :class="cn('flex shrink-0 items-center gap-1.5 text-xs', item.muted && 'opacity-45')"
      >
        <kbd
          v-for="k in item.keys"
          :key="k"
          aria-hidden="true"
          :class="
            cn(
              'inline-flex h-5 items-center justify-center rounded border border-border border-b-2 bg-background px-1.5 font-mono text-[11px] leading-none text-foreground',
              capWidth(k),
            )
          "
        >
          {{ cap(k) }}
        </kbd>
        <span class="whitespace-nowrap text-muted-foreground">{{ item.label }}</span>
      </span>
      </span>

      <button
        type="button"
        class="shrink-0 cursor-pointer self-center rounded p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
        :title="$t('keys.hide')"
        :aria-label="$t('keys.hide')"
        @click="setOpen(false)"
      >
        <X class="size-3.5" aria-hidden="true" />
      </button>
    </div>

    <!-- The way back. Small, but never nothing: a panel you cannot restore is
         a panel you should not have been able to dismiss. -->
    <button
      v-else
      type="button"
      class="pointer-events-auto flex cursor-pointer items-center gap-1 rounded-full border border-border-subtle bg-card/85 px-2.5 py-1 text-[11px] text-muted-foreground shadow-raised backdrop-blur-md hover:text-foreground"
      :title="$t('keys.show')"
      @click="setOpen(true)"
    >
      <ChevronUp class="size-3" aria-hidden="true" />
      {{ $t('keys.title') }}
    </button>
  </div>
</template>
