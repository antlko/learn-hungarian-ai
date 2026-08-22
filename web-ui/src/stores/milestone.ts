import { ref } from 'vue'
import { defineStore } from 'pinia'

/**
 * The three things the app is willing to celebrate. Each one corresponds to a
 * state change the backend can prove:
 *  · `unit`       — a curriculum unit finished
 *  · `topic`      — a grammar topic reaching `mastered`
 *  · `checkpoint` — an M1/M3/M6/M12/M18 assessment passed
 *
 * There is no `streak` kind and no `daily` kind. A streak is already visible on
 * Today and celebrating it would make the app's loudest moment the one with the
 * least learning behind it. [R14] .claude/research/14-motivation.md
 */
export type MilestoneKind = 'unit' | 'topic' | 'checkpoint'

export interface MilestoneEvent {
  kind: MilestoneKind
  /** Already translated. Callers have `$t`; the store does not. */
  title: string
  subtitle?: string
  /**
   * The measured fact behind the celebration, e.g. "юнит 3 из 24 — 32 слова в
   * колоде". REQUIRED, and required on purpose: a milestone with nothing
   * checkable behind it is a reward, and rewards are the part of gamification
   * the evidence does not support ([R14], [R16] §3). If you cannot state the
   * number, you do not have a milestone.
   */
  fact: string
}

interface ActiveMilestone extends MilestoneEvent {
  /** Monotonic, so re-firing the same milestone still re-triggers the overlay. */
  id: number
}

/**
 * Fire-and-forget celebration channel. Any screen can call `celebrate()`; a
 * single `Milestone.vue` mounted in the shell renders whatever is current.
 *
 * One at a time by design: a second milestone replaces the first rather than
 * queueing. Two celebrations stacking up would turn a rare event into a stream,
 * which is exactly what makes a reward feel cheap.
 */
export const useMilestoneStore = defineStore('milestone', () => {
  const current = ref<ActiveMilestone | null>(null)
  let seq = 0

  function celebrate(event: MilestoneEvent) {
    current.value = { ...event, id: ++seq }
  }

  function dismiss() {
    current.value = null
  }

  return { current, celebrate, dismiss }
})
