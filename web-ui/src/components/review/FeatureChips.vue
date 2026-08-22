<script setup lang="ts">
import { Check, X } from '@lucide/vue'
import type { FeatureGrade } from '@/types'

/**
 * The features this answer graded.
 *
 * This is the second knowledge state space made visible. Without it a learner
 * has no way to tell that answering one cloze credited their inessive AND their
 * possessive marking — the whole mechanism would be invisible, and an invisible
 * mechanism is one nobody trusts.
 */
defineProps<{ features: FeatureGrade[] }>()
</script>

<template>
  <div v-if="features.length" class="flex flex-col gap-1.5">
    <p class="text-xs text-muted-foreground">{{ $t('review.graded') }}</p>
    <div class="flex flex-wrap gap-1.5">
      <span
        v-for="f in features"
        :key="f.FeatureID"
        class="inline-flex items-center gap-1 rounded-md border px-2 py-0.5 font-mono text-xs"
        :class="
          f.Correct
            ? 'border-known/40 bg-known/10 text-known'
            : 'border-error/40 bg-error/10 text-error'
        "
      >
        <Check v-if="f.Correct" class="size-3" aria-hidden="true" />
        <X v-else class="size-3" aria-hidden="true" />
        {{ f.FeatureID }}
      </span>
    </div>
  </div>
</template>
