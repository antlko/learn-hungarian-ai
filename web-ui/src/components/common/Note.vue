<script setup lang="ts">
/**
 * The calm panel — how this app says something it cannot or will not do.
 *
 * The product is full of these: a topic with no drill generator, an ear session
 * refused below the talker floor, a coverage warning, words the pack has no
 * attested sentence for, a pack with no course in it. Every one of them is a
 * DECISION (see the honesty rules in CLAUDE.md), and before this component they
 * were rendered in `text-uncertain` or `text-error` from edge to edge — which
 * made a deliberate refusal look exactly like a crash. A learner cannot be
 * expected to trust an app that reports its own integrity as a fault.
 *
 * So the treatment is deliberately quiet, and colour is confined to the icon:
 *
 *   info   neutral. An empty state, a hint, a thing not started yet.
 *   limit  a stated limitation or refusal. The `uncertain` hue, on the icon
 *          only — the same hue the app uses everywhere else for "we will not
 *          claim this", and nowhere else does it mean "broken".
 *   error  something actually failed. The only variant that gets `destructive`,
 *          which is what keeps `destructive` meaning anything.
 *
 * The body stays at `muted-foreground` (8.35 on card in dark, 6.82 in light) and
 * an optional title at full `foreground`: readable at a glance, quiet at a
 * distance, never shouting.
 */
import { computed } from 'vue'
import { CircleAlert, CircleDashed, Info } from '@lucide/vue'
import { cn } from '@/lib/utils'

// The file is `Note.vue` because that is what it renders; the registered name
// is two words to satisfy `vue/multi-word-component-names`, same as Milestone.
defineOptions({ name: 'NotePanel' })

const props = withDefaults(
  defineProps<{
    variant?: 'info' | 'limit' | 'error'
    title?: string
    /** Drop the border and surface — for a note that already sits inside a card. */
    bare?: boolean
    class?: string
  }>(),
  { variant: 'info', bare: false },
)

const meta = computed(
  () =>
    ({
      info: { icon: Info, tone: 'text-muted-foreground' },
      limit: { icon: CircleDashed, tone: 'text-uncertain' },
      error: { icon: CircleAlert, tone: 'text-destructive' },
    })[props.variant],
)

// A refusal is not an interruption: only a real failure is announced assertively.
const role = computed(() => (props.variant === 'error' ? 'alert' : 'status'))
</script>

<template>
  <div
    :role="role"
    :class="
      cn(
        'flex items-start gap-3 text-sm leading-relaxed',
        !bare && 'rounded-xl border border-border-subtle bg-card px-4 py-3.5',
        $props.class,
      )
    "
  >
    <component :is="meta.icon" :class="cn('mt-0.5 size-4 shrink-0', meta.tone)" aria-hidden="true" />
    <div class="flex min-w-0 flex-1 flex-col gap-1.5">
      <p v-if="title" class="font-medium text-foreground">{{ title }}</p>
      <div class="text-muted-foreground"><slot /></div>
      <slot name="action" />
    </div>
  </div>
</template>
