<script setup lang="ts">
import { GraduationCap, ExternalLink } from '@lucide/vue'
import { buttonVariants } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import TutorConversation from './TutorConversation.vue'

/**
 * The grammar tutor, embedded under one topic's lesson. ADR-027, ADR-029.
 *
 * Placed AFTER the lesson and never before it — [R4] is the order of this
 * screen, and the tutor does not reorder it. The conversation is the
 * learner's latest chat pinned to this topic, or a new one pinned to it on
 * the first question; `TutorConversation.vue` is shared with the AI-Teacher
 * page, and the link opens that same chat there — never a second one.
 */
defineProps<{ topic: string; title: string }>()
</script>

<template>
  <Card variant="quiet" data-tutor-panel>
    <CardHeader class="flex flex-col gap-1.5 pb-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <CardTitle class="flex items-center gap-2 text-base">
          <GraduationCap class="size-4 text-primary" aria-hidden="true" />
          {{ $t('tutor.title') }}
        </CardTitle>
        <!-- The same chat, full-screen, beside every other one. -->
        <RouterLink
          :to="{ path: '/tutor', query: { topic } }"
          :class="buttonVariants({ variant: 'ghost', size: 'sm' })"
          class="gap-1.5 text-xs text-muted-foreground"
          :title="$t('tutor.openFull')"
        >
          <ExternalLink class="size-3.5" aria-hidden="true" />
          <span class="hidden sm:inline">{{ $t('tutor.openFull') }}</span>
        </RouterLink>
      </div>
      <p class="max-w-prose text-xs leading-relaxed text-muted-foreground">{{ $t('tutor.lead') }}</p>
    </CardHeader>

    <CardContent>
      <TutorConversation :topic="topic" compact />
    </CardContent>
  </Card>
</template>
