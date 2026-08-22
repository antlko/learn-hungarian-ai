<script setup lang="ts">
import { computed, ref } from 'vue'
import { BookOpen, BrainCircuit, Check, PencilRuler, UserRound } from '@lucide/vue'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth'

/**
 * The home screen is the study plan, not a dashboard of the app's own features.
 *
 * The four loops come from the source brief §3. Loop 4 is deliberately different
 * from the other three: the app CANNOT complete it, only the learner can tick it
 * manually, and the reason is stated on screen rather than hidden in docs.
 * See docs/PEDAGOGY.md and .claude/research/16-debunked.md §7.
 */
const auth = useAuthStore()

const dailyMinutes = computed(() => auth.settings?.review.dailyMinutes ?? 90)

// Budgets are proportions of the learner's stated daily minutes, matching the
// brief's 20-25 / 25-40 / 30 split rather than fixed numbers.
const loops = computed(() => [
  {
    key: 'memory',
    icon: BrainCircuit,
    budget: Math.round(dailyMinutes.value * 0.28),
    done: 0,
  },
  { key: 'input', icon: BookOpen, budget: Math.round(dailyMinutes.value * 0.39), done: 0 },
  { key: 'study', icon: PencilRuler, budget: Math.round(dailyMinutes.value * 0.33), done: 0 },
])

const humanDone = ref(false)
const packMissing = computed(() => auth.bootstrap?.packPresent === false)
</script>

<template>
  <div class="mx-auto flex w-full max-w-4xl flex-col gap-6">
    <div class="flex items-baseline justify-between">
      <h1 class="text-2xl font-semibold tracking-tight">{{ $t('today.title') }}</h1>
      <span v-if="packMissing" class="text-xs text-uncertain">
        {{ $t('today.empty.packMissing') }}
      </span>
    </div>

    <div class="grid gap-4 sm:grid-cols-2">
      <Card v-for="(loop, i) in loops" :key="loop.key">
        <CardContent class="flex flex-col gap-2 p-5">
          <div class="flex items-center gap-2 text-sm font-medium">
            <component :is="loop.icon" class="size-4 text-muted-foreground" aria-hidden="true" />
            <span class="text-muted-foreground">{{ i + 1 }}</span>
            {{ $t(`today.loops.${loop.key}`) }}
          </div>
          <p class="text-2xl font-semibold tabular-nums">
            {{ loop.done }}<span class="text-muted-foreground">/{{ loop.budget }}</span>
            <span class="ml-1 text-sm font-normal text-muted-foreground">
              {{ $t('common.min') }}
            </span>
          </p>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ $t(`today.loopHints.${loop.key}`) }}
          </p>
        </CardContent>
      </Card>

      <!-- Loop 4: the app never marks this complete. -->
      <Card class="border-dashed">
        <CardContent class="flex flex-col gap-2 p-5">
          <div class="flex items-center gap-2 text-sm font-medium">
            <UserRound class="size-4 text-muted-foreground" aria-hidden="true" />
            <span class="text-muted-foreground">4</span>
            {{ $t('today.loops.human') }}
          </div>
          <p class="text-xs leading-relaxed text-muted-foreground">
            {{ $t('today.loopHints.human') }}
          </p>
          <p class="text-xs leading-relaxed text-uncertain">{{ $t('today.humanNotice') }}</p>
          <Button
            :variant="humanDone ? 'secondary' : 'outline'"
            size="sm"
            class="mt-1 self-start"
            @click="humanDone = !humanDone"
          >
            <Check v-if="humanDone" class="size-3.5" />
            {{ $t('today.markDone') }}
          </Button>
        </CardContent>
      </Card>
    </div>

    <Card>
      <CardContent class="flex flex-col gap-1.5 p-5">
        <p class="text-sm font-medium">{{ $t('today.empty.title') }}</p>
        <p class="text-sm leading-relaxed text-muted-foreground">{{ $t('today.empty.body') }}</p>
      </CardContent>
    </Card>
  </div>
</template>
