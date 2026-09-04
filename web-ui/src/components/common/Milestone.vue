<script setup lang="ts">
/**
 * The celebration surface — and the boundary around it.
 *
 * [R14] Gamification reliably improves motivation, autonomy and relatedness, and
 * has **minimal impact on competence** — see .claude/research/14-motivation.md.
 * So we take the motivational mechanic and refuse the substitution: this
 * component may only be fired by something the backend can prove happened. A
 * unit finished, a topic reaching `mastered`, a checkpoint passed. The
 * `fact` field is required by the store's type for that reason — the overlay
 * always shows the real number behind the moment, never the moment alone.
 *
 * [R16] §3, .claude/research/16-debunked.md — what was deliberately NOT built
 * here, so that nobody later reads the absence as an oversight and "fixes" it:
 *
 *   · XP, points, scores        · levels, ranks, tiers
 *   · leagues, leaderboards     · gems, coins, any invented currency
 *   · streak freezes, repairs   · lootboxes, chests, random rewards
 *   · combo/multiplier counters · daily-login celebrations
 *
 * Every one of those is an extrinsic economy, which is the part of gamification
 * most likely to crowd out the actual goal, and the part with no competence
 * evidence behind it. The streak the app does keep counts obligations met and
 * lives on Today, quietly; it is not celebrated here.
 *
 * Three hard behaviours, all verifiable by reading this file:
 *   1. It never blocks input. The container and the panel are
 *      `pointer-events-none`; only the dismiss button takes pointers, and
 *      focus is never stolen — a learner mid-answer keeps typing through it.
 *   2. It honours reduced motion. The `animate-*` classes are neutralised by
 *      the global rule in `assets/tailwind.css`, and `motionDuration` collapses
 *      the exit wait so nothing lingers waiting for an animation that is not
 *      playing. The hold time is NOT shortened: reading still takes as long.
 *   3. It honours the sound setting. `play()` is a no-op when sound is off.
 */
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Milestone as MilestoneIcon, X } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { MOTION, motionDuration } from '@/lib/motion'
import { play } from '@/lib/sound'
import { useMilestoneStore } from '@/stores/milestone'

// The file is `Milestone.vue` because that is what it renders; the registered
// name is two words to satisfy `vue/multi-word-component-names`.
defineOptions({ name: 'MilestoneOverlay' })

/**
 * Long enough to read three short lines without hurrying, short enough that it
 * is gone before the next card matters. It self-dismisses either way — a
 * celebration you have to close is a chore.
 */
const HOLD_MS = 5200

const store = useMilestoneStore()
const current = computed(() => store.current)
const leaving = ref(false)

let holdTimer: ReturnType<typeof setTimeout> | undefined
let exitTimer: ReturnType<typeof setTimeout> | undefined

function clearTimers() {
  clearTimeout(holdTimer)
  clearTimeout(exitTimer)
}

function close() {
  clearTimers()
  leaving.value = true
  exitTimer = setTimeout(() => {
    leaving.value = false
    store.dismiss()
  }, motionDuration(MOTION.cardOut))
}

watch(
  () => current.value?.id,
  (id) => {
    clearTimers()
    if (id === undefined) return
    leaving.value = false
    play('milestone')
    holdTimer = setTimeout(close, HOLD_MS)
  },
)

onBeforeUnmount(clearTimers)
</script>

<template>
  <Teleport to="body">
    <!--
      `role="status"` + `aria-live="polite"` announces the milestone after the
      current utterance rather than interrupting it — same rule as the visual
      one: never cut across what the learner is doing.
    -->
    <div
      v-if="current"
      class="pointer-events-none fixed inset-x-0 bottom-6 z-50 flex justify-center px-4 md:left-(--rail)"
      role="status"
      aria-live="polite"
    >
      <div
        :key="current.id"
        class="pointer-events-none flex w-full max-w-md items-start gap-3 rounded-xl border border-border bg-popover px-4 py-3 text-popover-foreground shadow-lg"
        :class="leaving ? 'animate-card-out' : 'animate-milestone'"
      >
        <MilestoneIcon class="mt-0.5 size-5 shrink-0 text-primary" aria-hidden="true" />

        <div class="flex min-w-0 flex-1 flex-col gap-0.5">
          <p class="text-sm font-semibold">{{ current.title }}</p>
          <p v-if="current.subtitle" class="text-xs text-muted-foreground">
            {{ current.subtitle }}
          </p>
          <!-- The measured fact. Tabular figures so the numbers do not wobble. -->
          <p class="mt-1 text-xs tabular-nums text-foreground">{{ current.fact }}</p>
        </div>

        <Button
          variant="ghost"
          size="icon"
          class="pointer-events-auto -mt-1 -mr-1 size-7 shrink-0"
          :aria-label="$t('common.dismiss')"
          :title="$t('common.dismiss')"
          @click="close"
        >
          <X class="size-4" />
        </Button>
      </div>
    </div>
  </Teleport>
</template>
