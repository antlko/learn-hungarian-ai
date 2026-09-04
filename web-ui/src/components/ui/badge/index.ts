import { cva, type VariantProps } from 'class-variance-authority'

export { default as Badge } from './Badge.vue'

/**
 * One pill, six meanings. Five screens had grown five ad-hoc `<span>` recipes
 * for the same object; this is the single copy. The tones are the knowledge
 * palette (docs/UX.md §11) — colour is never the only channel, so a badge
 * carries text or a number, never a bare dot.
 */
export const badgeVariants = cva(
  'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-semibold tabular-nums whitespace-nowrap',
  {
    variants: {
      tone: {
        neutral: 'bg-muted text-muted-foreground',
        learning: 'bg-learning/15 text-learning',
        new: 'bg-new/15 text-new',
        known: 'bg-known/15 text-known',
        error: 'bg-error/15 text-error',
        uncertain: 'bg-uncertain/15 text-uncertain',
        outline: 'border border-border text-muted-foreground font-normal',
      },
    },
    defaultVariants: { tone: 'neutral' },
  },
)

export type BadgeVariants = VariantProps<typeof badgeVariants>
