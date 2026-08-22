<script setup lang="ts">
import { ArrowRight, Flag } from '@lucide/vue'
import { buttonVariants } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'

/**
 * The end of the prescribed course.
 *
 * The hand-off is the design, not a limitation. [R11] is about volume of
 * SELF-CHOSEN input, and a course that never ends produces a learner who can
 * only read material made for them — so the app stops prescribing vocabulary and
 * points at real sources instead. See docs/PEDAGOGY.md §2.8 and
 * .claude/research/11-extensive-reading.md.
 */
defineProps<{ completed: number }>()
</script>

<template>
  <Card variant="raised" class="border-known/50 bg-known/5">
    <CardHeader class="flex flex-col gap-2">
      <span class="flex items-center gap-2 text-xs font-semibold tracking-widest text-known uppercase">
        <Flag class="size-4" aria-hidden="true" />
        {{ $t('plan.handOver.badge') }}
      </span>
      <CardTitle class="text-2xl leading-tight">{{ $t('plan.handOver.title') }}</CardTitle>
      <p class="text-sm leading-relaxed">
        {{ $t('plan.handOver.done', { n: completed }) }}
      </p>
    </CardHeader>

    <CardContent class="flex flex-col gap-4">
      <p class="text-sm leading-relaxed text-muted-foreground">{{ $t('plan.handOver.why') }}</p>
      <p class="text-sm leading-relaxed text-muted-foreground">{{ $t('plan.handOver.next') }}</p>

      <RouterLink
        :to="{ name: 'resources' }"
        :class="cn(buttonVariants({ size: 'lg' }), 'self-start')"
      >
        {{ $t('plan.handOver.toResources') }}
        <ArrowRight class="size-4" aria-hidden="true" />
      </RouterLink>
    </CardContent>
  </Card>
</template>
