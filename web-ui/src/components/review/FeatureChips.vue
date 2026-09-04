<script setup lang="ts">
import { onMounted } from 'vue'
import { Check, X } from '@lucide/vue'
import { useFeaturesStore } from '@/stores/features'
import type { FeatureGrade } from '@/types'

/**
 * The features this answer graded.
 *
 * This is the second knowledge state space made visible. Without it a learner
 * has no way to tell that answering one cloze credited their inessive AND their
 * possessive marking — the whole mechanism would be invisible, and an invisible
 * mechanism is one nobody trusts.
 *
 * It only works if the chip says something. It used to print the raw feature
 * id — `Case=Acc`, `Number=Sing` — which names the mechanism to us and nothing
 * at all to the learner. The pack ships a label for every feature in both
 * locales; the id stays, small and beneath, because it is what the heatmap and
 * the topic list are keyed on and hiding it entirely would make two screens
 * impossible to connect.
 */
defineProps<{ features: FeatureGrade[] }>()

const catalogue = useFeaturesStore()
onMounted(() => void catalogue.ensure())
</script>

<template>
  <div v-if="features.length" class="flex flex-col gap-1.5">
    <p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
      {{ $t('review.graded') }}
    </p>
    <div class="flex flex-wrap gap-1.5">
      <span
        v-for="f in features"
        :key="f.FeatureID"
        class="inline-flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs"
        :class="
          f.Correct
            ? 'border-known/40 bg-known/10 text-known'
            : 'border-error/40 bg-error/10 text-error'
        "
      >
        <Check v-if="f.Correct" class="size-3 shrink-0" aria-hidden="true" />
        <X v-else class="size-3 shrink-0" aria-hidden="true" />
        <span>{{ catalogue.label(f.FeatureID) }}</span>
        <span class="font-mono text-[10px] opacity-60">{{ f.FeatureID }}</span>
      </span>
    </div>
  </div>
</template>
