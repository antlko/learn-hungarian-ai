import type { Topic } from '@/types'

/**
 * The learner-facing stage of a topic.
 *
 * The server's phase says what the GATE decided; this adds the one distinction
 * the gate does not draw — «ещё не начато» — because `blocked` is the default of
 * every drillable topic ([R5]) and so says nothing about whether anything was
 * answered. Display only: it never decides what is practised, and ADR-043's
 * "which tab opens first" does not read it (it reads attempts, never the phase).
 */
export type TopicStage = 'locked' | 'ruleOnly' | 'fresh' | 'oneAtATime' | 'mixed' | 'mastered'

export function topicStage(t: Pick<Topic, 'phase' | 'progress'>): TopicStage {
  switch (t.phase) {
    case 'locked':
      return 'locked'
    case 'explained':
      return 'ruleOnly'
    case 'mastered':
      return 'mastered'
    case 'interleaved':
      return 'mixed'
    default:
      return t.progress.attempts > 0 ? 'oneAtATime' : 'fresh'
  }
}
