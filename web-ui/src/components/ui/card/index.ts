import { cva, type VariantProps } from 'class-variance-authority'

export { default as Card } from './Card.vue'
export { default as CardHeader } from './CardHeader.vue'
export { default as CardTitle } from './CardTitle.vue'
export { default as CardDescription } from './CardDescription.vue'
export { default as CardContent } from './CardContent.vue'
export { default as CardFooter } from './CardFooter.vue'

/**
 * The four surface tiers. The reasoning, with the contrast measurements behind
 * it, is the "Elevation" block in `assets/tailwind.css`; the short version is
 * that every tier keeps the `card` fill and differs only in its boundary, so
 * nothing sitting on a card ever loses contrast by being promoted.
 *
 * Choosing one:
 *
 *   raised   the primary object of the screen. ONE per screen — a second is a
 *            claim that two things are both the most important, which is a
 *            claim that neither is.
 *   default  a thing you can click. Keeps the 3:1 boundary because WCAG 1.4.11
 *            applies to a component, and adds a hover tint.
 *   quiet    a thing you only read. The decorative boundary tier: present, but
 *            it does not compete with the object above it.
 *   inset    a well cut into the surrounding plane — an audio player, a
 *            transcript, a result panel inside the card that produced it.
 */
export const cardVariants = cva('rounded-xl text-card-foreground', {
  variants: {
    variant: {
      raised: 'border border-border bg-card shadow-raised',
      default: 'border border-border bg-card',
      quiet: 'border border-border-subtle bg-card',
      inset: 'border border-border-subtle bg-background',
      /** Soft coloured card (ADR-059): tint fill + 1px tinted edge, no shadow. */
      'tint-mist': 'border border-tint-mist-edge bg-tint-mist',
      'tint-cream': 'border border-tint-cream-edge bg-tint-cream',
      'tint-blush': 'border border-tint-blush-edge bg-tint-blush',
      'tint-sage': 'border border-tint-sage-edge bg-tint-sage',
    },
    /**
     * Clickable cards get the hover tint and the pointer. It is a separate axis
     * rather than a fifth variant because any tier can be clickable.
     */
    interactive: {
      true: 'cursor-pointer transition-colors hover:bg-accent/40 hover:border-primary/50',
      false: '',
    },
  },
  /*
    A clickable card is a component, so WCAG 1.4.11 applies to its boundary and
    the decorative tier will not do. Promoting it here rather than in the
    `interactive` variant means the class order is decided by cva instead of by
    whichever of two border colours `twMerge` happens to resolve last.
  */
  compoundVariants: [
    { variant: 'quiet', interactive: true, class: 'border-border' },
    { variant: 'inset', interactive: true, class: 'border-border' },
    // A clickable tint card keeps the 3:1 boundary, like every other tier.
    { variant: 'tint-mist', interactive: true, class: 'border-border' },
    { variant: 'tint-cream', interactive: true, class: 'border-border' },
    { variant: 'tint-blush', interactive: true, class: 'border-border' },
    { variant: 'tint-sage', interactive: true, class: 'border-border' },
  ],
  defaultVariants: { variant: 'default', interactive: false },
})

export type CardVariants = VariantProps<typeof cardVariants>
