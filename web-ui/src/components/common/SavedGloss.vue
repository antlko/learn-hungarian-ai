<script setup lang="ts">
import { computed, ref } from 'vue'
import { toast } from 'vue-sonner'
import { Check, Trash2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { describeError } from '@/lib/errors'
import { api } from '@/api'
import type { UserGloss } from '@/types'

/**
 * A meaning from the learner's own dictionary (`user_gloss`), on any word card.
 *
 * One component for the reader's rail and the review card's inspector: the
 * second never read `user_gloss` at all, so a meaning saved from the AI entry
 * vanished the next time the learner clicked the same word — the save had
 * worked, the card simply did not look.
 *
 * The badge says whose line this is, beside the text and never in a tooltip:
 * «ваш словарь» for the learner's own, «машинное · <model>» for one a model
 * wrote. A machine line stays labelled after it is saved and does not reach a
 * card's back until «Принять как своё» re-saves it as `manual`. [ADR-014],
 * [ADR-035], [ADR-042]
 */
const props = defineProps<{ form: string; gloss: UserGloss }>()
const emit = defineEmits<{ change: [gloss: UserGloss | null] }>()

const busy = ref(false)
const model = computed(() => (props.gloss.model ?? '').split('/').pop() ?? '')

async function accept() {
  if (busy.value) return
  busy.value = true
  try {
    emit('change', await api.saveGloss(props.form, { gloss: props.gloss.gloss, source: 'manual' }))
  } catch (e) {
    toast.error(describeError(e))
  } finally {
    busy.value = false
  }
}

async function forget() {
  if (busy.value) return
  busy.value = true
  try {
    await api.deleteGloss(props.form)
    emit('change', null)
  } catch (e) {
    toast.error(describeError(e))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-1">
    <p class="flex items-center gap-2 text-xs text-muted-foreground">
      {{ $t('reader.meaning') }}
      <Badge v-if="gloss.source === 'machine'" tone="uncertain" class="text-[10px]">
        {{ $t('reader.savedBadgeMachine', { model }) }}
      </Badge>
      <Badge v-else tone="known" class="text-[10px]">{{ $t('reader.yourDictionary') }}</Badge>
      <button
        type="button"
        class="ml-auto cursor-pointer text-[10px] text-muted-foreground hover:text-error"
        :title="$t('reader.forgetGloss')"
        :aria-label="$t('reader.forgetGloss')"
        :disabled="busy"
        @click="forget"
      >
        <Trash2 class="size-3" aria-hidden="true" />
      </button>
    </p>
    <ol class="list-inside list-decimal text-sm leading-relaxed">
      <li>{{ gloss.gloss }}</li>
    </ol>
    <template v-if="gloss.source === 'machine'">
      <p class="text-[10px] leading-relaxed text-uncertain">
        {{ $t('translation.machine', { model }) }}
      </p>
      <!-- The card's back is graded against, so a machine line does not get
           there on its own. One person-shaped click is what makes it a claim
           somebody stands behind. -->
      <p class="text-[10px] leading-relaxed text-muted-foreground">
        {{ $t('reader.machineNotOnCards') }}
      </p>
      <Button size="sm" variant="outline" class="mt-1 self-start" :disabled="busy" @click="accept">
        <Check class="size-3.5" aria-hidden="true" />
        {{ $t('reader.acceptAsMine') }}
      </Button>
    </template>
  </div>
</template>
