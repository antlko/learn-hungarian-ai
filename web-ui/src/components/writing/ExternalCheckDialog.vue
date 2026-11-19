<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Copy } from '@lucide/vue'
import BottomSheet from '@/components/common/BottomSheet.vue'
import Note from '@/components/common/Note.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { writingApi } from '@/api'
import { describeError } from '@/lib/errors'
import { copyText, selectAll } from '@/lib/writing'
import { useUiStore } from '@/stores/ui'
import type { WritingCheck, WritingPromptResponse } from '@/types'

/**
 * The outside-chat check in two steps (docs/UX.md §7.3). Step 1 builds the
 * prompt on the server (after the draft is flushed, so the chat gets what is on
 * screen) and offers it to copy; step 2 takes the chat's answer. An answer the
 * server cannot read keeps the paste where it is — nothing is stored.
 */
const props = defineProps<{
  open: boolean
  pieceId: number
  /** Saves the draft; awaited before the prompt is built. */
  flush: () => Promise<void>
}>()
const emit = defineEmits<{ close: []; done: [check: WritingCheck] }>()
const { t } = useI18n()
const ui = useUiStore()

const step = ref<1 | 2>(1)
const prompt = ref<WritingPromptResponse | null>(null)
const loading = ref(false)
const error = ref('')
const copied = ref<'' | 'ok' | 'fallback'>('')
const raw = ref('')
const chat = ref('')
const sending = ref(false)
const previewEl = ref<HTMLTextAreaElement | null>(null)

async function build() {
  loading.value = true
  error.value = ''
  prompt.value = null
  try {
    await props.flush()
    prompt.value = await writingApi.prompt(props.pieceId, ui.locale)
  } catch (e) {
    error.value = describeError(e)
  } finally {
    loading.value = false
  }
}

watch(
  () => props.open,
  (open) => {
    if (!open) return
    // The paste survives a close and reopen of the same dialog; the prompt is
    // rebuilt because the text may have changed meanwhile.
    step.value = raw.value ? 2 : 1
    copied.value = ''
    build()
  },
  { immediate: true },
)

async function copyPrompt() {
  if (!prompt.value) return
  if (await copyText(prompt.value.prompt)) {
    copied.value = 'ok'
  } else {
    copied.value = 'fallback'
    selectAll(previewEl.value)
  }
}

async function submit() {
  if (sending.value) return
  if (!raw.value.trim()) {
    error.value = t('writing.ext.emptyPaste')
    return
  }
  sending.value = true
  error.value = ''
  try {
    const check = await writingApi.external(props.pieceId, {
      lang: ui.locale,
      // The essay the prompt carried — the server anchors errors in THAT text.
      text: prompt.value?.text ?? '',
      raw: raw.value,
      model: chat.value.trim() || t('writing.ext.chatDefault'),
    })
    raw.value = ''
    chat.value = ''
    emit('done', check)
  } catch (e) {
    // The paste stays: the learner can fix it or paste again.
    error.value = describeError(e)
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <BottomSheet dialog tall :open="open" :title="t('writing.ext.title')" @close="emit('close')">
    <div class="flex flex-col gap-3 pb-2">
      <Note v-if="error" variant="error">{{ error }}</Note>

      <template v-if="step === 1">
        <h3 class="text-sm font-medium">{{ t('writing.ext.step1') }}</h3>
        <p class="text-muted-foreground">{{ t('writing.ext.step1Body') }}</p>
        <p v-if="loading" class="text-muted-foreground" role="status">{{ t('writing.ext.preparing') }}</p>
        <template v-else-if="prompt">
          <Label for="writing-prompt" class="sr-only">{{ t('writing.ext.previewLabel') }}</Label>
          <textarea
            id="writing-prompt"
            ref="previewEl"
            :value="prompt.prompt"
            readonly
            rows="9"
            class="w-full resize-y rounded-md border border-input bg-background p-2.5 font-mono text-xs leading-relaxed focus-visible:outline-2 focus-visible:outline-ring"
          />
          <div class="flex flex-wrap items-center gap-3">
            <Button @click="copyPrompt">
              <component :is="copied === 'ok' ? Check : Copy" class="size-4" aria-hidden="true" />
              {{ copied === 'ok' ? t('writing.ext.copied') : t('writing.ext.copy') }}
            </Button>
            <p v-if="copied === 'fallback'" class="text-xs text-muted-foreground" role="status">{{ t('writing.ext.copyFallback') }}</p>
          </div>
          <p class="text-xs text-muted-foreground">{{ t('writing.ext.codeNote') }}</p>
        </template>
      </template>

      <template v-else>
        <h3 class="text-sm font-medium">{{ t('writing.ext.step2') }}</h3>
        <div class="flex flex-col gap-1.5">
          <Label for="writing-paste">{{ t('writing.ext.pasteLabel') }}</Label>
          <textarea
            id="writing-paste"
            v-model="raw"
            rows="10"
            :placeholder="t('writing.ext.pastePlaceholder')"
            spellcheck="false"
            autocapitalize="off"
            class="w-full resize-y rounded-md border border-input bg-transparent p-2.5 text-sm leading-relaxed placeholder:text-muted-foreground focus-visible:outline-2 focus-visible:outline-ring"
          />
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="writing-chat">{{ t('writing.ext.chatLabel') }}</Label>
          <Input id="writing-chat" v-model="chat" maxlength="80" :placeholder="t('writing.ext.chatDefault')" />
          <p class="text-xs text-muted-foreground">{{ t('writing.ext.chatHint') }}</p>
        </div>
      </template>
    </div>

    <template #footer>
      <div class="flex flex-wrap justify-end gap-2">
        <template v-if="step === 1">
          <Button :disabled="!prompt" @click="step = 2">{{ t('writing.ext.next') }}</Button>
        </template>
        <template v-else>
          <Button variant="ghost" :disabled="sending" @click="step = 1">{{ t('writing.ext.back') }}</Button>
          <Button :disabled="sending || !raw.trim()" @click="submit">
            {{ sending ? t('writing.ext.submitting') : t('writing.ext.submit') }}
          </Button>
        </template>
      </div>
    </template>
  </BottomSheet>
</template>
