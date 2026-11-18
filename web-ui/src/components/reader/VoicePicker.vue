<script setup lang="ts">
import { computed } from 'vue'
import { Play, Square } from '@lucide/vue'
import BottomSheet from '@/components/common/BottomSheet.vue'
import { Button } from '@/components/ui/button'
import { PERSONAS, SAMPLE_TEXT } from '@/lib/tts'
import { useTtsStore } from '@/stores/tts'

/**
 * «Кем озвучить?» — the voice the reader speaks with, chosen once.
 *
 * Opens on the first Piper playback when no voice is stored, and from the audio
 * bar's voice menu and Settings. Three cards, each with a sample line in
 * Hungarian so a voice is judged by ear and not by name. The initial in the
 * circle is chrome: no portraits, no character, nothing to collect.
 *
 * Closing it without choosing cancels the playback that asked.
 */
const tts = useTtsStore()

const cards = computed(() =>
  PERSONAS.filter((p) => tts.voices.some((v) => v.id === p.id)),
)
</script>

<template>
  <BottomSheet dialog :open="tts.pickerOpen" :title="$t('tts.pick.title')" @close="tts.dismissPicker()">
    <p class="text-sm text-muted-foreground">{{ $t('tts.pick.hint') }}</p>
    <p class="mt-3 text-sm" lang="hu">{{ SAMPLE_TEXT }}</p>

    <ul class="mt-3 flex flex-col gap-2.5">
      <li
        v-for="p in cards"
        :key="p.id"
        class="flex flex-wrap items-center gap-3 rounded-xl border border-tint-mist-edge bg-card p-3"
        :class="tts.activeVoice === p.id ? 'ring-2 ring-primary' : ''"
      >
        <span
          class="flex size-10 shrink-0 items-center justify-center rounded-full bg-tint-mist font-display text-lg font-medium"
          aria-hidden="true"
          >{{ p.initial }}</span
        >
        <div class="min-w-0 flex-1">
          <p class="text-sm font-medium">{{ $t(`tts.voice.${p.id}`) }}</p>
          <p class="text-xs text-muted-foreground">{{ $t(`tts.gender.${p.gender}`) }}</p>
        </div>
        <div class="flex shrink-0 items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            class="max-md:min-h-11"
            :aria-label="`${$t('tts.pick.sample')}: ${$t(`tts.voice.${p.id}`)}`"
            @click="tts.sampling === p.id ? tts.dismissSample() : tts.playSample(p.id)"
          >
            <Square v-if="tts.sampling === p.id" class="size-3.5" aria-hidden="true" />
            <Play v-else class="size-3.5" aria-hidden="true" />
            {{ $t('tts.pick.sample') }}
          </Button>
          <Button type="button" size="sm" class="max-md:min-h-11" @click="tts.choose(p.id)">
            {{ $t('tts.pick.choose') }}
          </Button>
        </div>
      </li>
    </ul>
  </BottomSheet>
</template>
