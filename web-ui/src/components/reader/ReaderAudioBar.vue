<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Headphones, Pause, Play, Square } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/select'
import { LISTEN_MODE_READY, PERSONAS, RATES } from '@/lib/tts'
import { useSpeechStore } from '@/stores/speech'
import { useTtsStore } from '@/stores/tts'
import type { TtsVoiceId } from '@/types'

/**
 * The reader's audio bar: listen to the whole text from the selected sentence,
 * with the voice and the speed in reach.
 *
 * The same buttons work with and without the Piper engine; without it the
 * device voice reads and the voice menu is replaced by one honest line, so the
 * bar never looks like it offers choices it does not have. In listening mode
 * (`?mode=listen`) playback and speed belong to `ListenMode`, so only the voice
 * and the way back to plain reading stay here.
 */
defineProps<{
  /** There is something to read: the text has loaded and has sentences. */
  canPlay: boolean
}>()
const emit = defineEmits<{ play: [] }>()

const tts = useTtsStore()
const speech = useSpeechStore()
const route = useRoute()
const router = useRouter()

const idle = computed(() => tts.status === 'idle')
const canSpeak = computed(() => tts.piperReady || speech.usable)
const playTitle = computed(() => {
  if (canSpeak.value) return undefined
  if (!speech.supported) return 'speech.unsupported'
  if (!speech.available) return 'speech.noVoice'
  return 'speech.off'
})

const listening = computed(() => route.query.mode === 'listen')
function toggleMode() {
  const query = { ...route.query }
  if (listening.value) delete query.mode
  else query.mode = 'listen'
  void router.replace({ query })
}

function onVoice(v: string | undefined) {
  if (v) tts.setVoice(v as TtsVoiceId)
}
const offered = computed(() => PERSONAS.filter((p) => tts.voices.some((v) => v.id === p.id)))
</script>

<template>
  <div
    class="sticky top-0 z-20 flex flex-wrap items-center gap-2 rounded-xl border border-tint-mist-edge bg-card/95 p-2 shadow-sm backdrop-blur max-md:-mx-4 max-md:rounded-none max-md:border-x-0"
    role="group"
    :aria-label="$t('reader.audio.label')"
  >
    <Button
      v-if="idle && !listening"
      type="button"
      size="sm"
      class="max-md:min-h-11"
      :disabled="!canPlay || !canSpeak"
      :title="playTitle ? $t(playTitle) : undefined"
      @click="emit('play')"
    >
      <Play class="size-3.5" aria-hidden="true" />
      {{ $t('reader.audio.playAll') }}
    </Button>
    <template v-else-if="!listening">
      <Button
        v-if="tts.status === 'playing'"
        type="button"
        size="sm"
        class="max-md:min-h-11"
        @click="tts.pause()"
      >
        <Pause class="size-3.5" aria-hidden="true" />
        {{ $t('reader.audio.pause') }}
      </Button>
      <Button v-else type="button" size="sm" class="max-md:min-h-11" @click="tts.resume()">
        <Play class="size-3.5" aria-hidden="true" />
        {{ $t('reader.audio.resume') }}
      </Button>
      <Button type="button" size="sm" variant="outline" class="max-md:min-h-11" @click="tts.stop()">
        <Square class="size-3.5" aria-hidden="true" />
        {{ $t('reader.audio.stop') }}
      </Button>
    </template>

    <label v-if="!listening" class="flex items-center gap-1.5 text-xs text-muted-foreground">
      <span>{{ $t('reader.audio.speed') }}</span>
      <Select
        class="h-8 w-[5.5rem] max-md:h-11"
        :model-value="String(tts.rate)"
        @update:model-value="(v) => tts.setRate(Number(v))"
      >
        <option v-for="r in RATES" :key="r" :value="String(r)">{{ r }}×</option>
      </Select>
    </label>

    <label v-if="tts.piperReady" class="flex items-center gap-1.5 text-xs text-muted-foreground">
      <span>{{ $t('reader.audio.voice') }}</span>
      <Select
        class="h-8 w-auto max-md:h-11"
        :model-value="tts.activeVoice"
        @update:model-value="onVoice"
      >
        <option value="" disabled>{{ $t('tts.pick.choose') }}</option>
        <option v-for="p in offered" :key="p.id" :value="p.id">
          {{ $t(`tts.voice.${p.id}`) }} · {{ $t(`tts.gender.${p.gender}`) }}
        </option>
      </Select>
    </label>
    <span v-else class="text-xs text-muted-foreground" :title="$t('tts.unavailable')">
      {{ $t('tts.systemVoice') }}
    </span>

    <Button
      v-if="LISTEN_MODE_READY"
      type="button"
      size="sm"
      :variant="listening ? 'secondary' : 'outline'"
      :aria-pressed="listening"
      class="ml-auto max-md:min-h-11"
      @click="toggleMode"
    >
      <Headphones class="size-3.5" aria-hidden="true" />
      {{ listening ? $t('reader.audio.exitListenMode') : $t('reader.audio.listenMode') }}
    </Button>
  </div>
</template>
