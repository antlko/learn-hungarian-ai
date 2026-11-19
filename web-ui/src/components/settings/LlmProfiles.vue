<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Check, Pencil, Trash2, X } from '@lucide/vue'
import { api } from '@/api'
import { describeError } from '@/lib/errors'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import type { LLMProfile, LLMProfileRequest, Service } from '@/types'

/**
 * Saved tutors (ADR-082): named presets of the ИИ-учитель's server and model.
 * Switching copies the preset into the one active tutor on the server, so
 * everything that talks to the tutor is unchanged. Admin-only: config is global.
 * The API key is one shared secret and is not part of a preset.
 */
const props = defineProps<{
  profiles: LLMProfile[]
  active: string
  current: Service
  saving?: boolean
}>()
const emit = defineEmits<{ changed: [] }>()

const { t } = useI18n()
const mode = ref<'save' | 'rename' | null>(null)
const name = ref('')
const busy = ref(false)

const activeProfile = computed(() => props.profiles.find((p) => p.id === props.active))
const full = computed(() => props.profiles.length >= 8)

async function run(fn: () => Promise<unknown>, done = true) {
  busy.value = true
  try {
    await fn()
    mode.value = null
    emit('changed')
    if (done) toast.success(t('common.saved'))
  } catch (e) {
    toast.error(describeError(e))
  } finally {
    busy.value = false
  }
}

function onPick(e: Event) {
  const id = (e.target as HTMLSelectElement).value
  if (id === '') return
  void run(() => api.activateLlmProfile(id))
}

function startSave() {
  name.value = ''
  mode.value = 'save'
}

function startRename() {
  name.value = activeProfile.value?.name ?? ''
  mode.value = 'rename'
}

function submit() {
  const n = name.value.trim()
  if (!n) return
  if (mode.value === 'rename' && activeProfile.value) {
    const p = activeProfile.value
    const body: LLMProfileRequest = {
      name: n,
      provider: p.provider,
      endpoint: p.endpoint,
      model: p.model ?? '',
    }
    void run(() => api.updateLlmProfile(p.id, body))
    return
  }
  const body: LLMProfileRequest = {
    name: n,
    provider: props.current.provider,
    endpoint: props.current.endpoint,
    model: props.current.model ?? '',
  }
  void run(() => api.createLlmProfile(body))
}

function remove() {
  const p = activeProfile.value
  if (p) void run(() => api.deleteLlmProfile(p.id))
}
</script>

<template>
  <div class="flex flex-col gap-2 rounded-md border border-border/60 p-3">
    <div class="grid items-end gap-3 sm:grid-cols-[10rem_1fr_auto]">
      <Label for="llm-profile" class="pb-2">{{ $t('settings.llmProfiles.label') }}</Label>
      <Select
        v-if="profiles.length"
        id="llm-profile"
        :model-value="active"
        :disabled="busy || saving"
        @change="onPick"
      >
        <option v-if="!active" value="">{{ $t('settings.llmProfiles.unsaved') }}</option>
        <option v-for="p in profiles" :key="p.id" :value="p.id">{{ p.name }}</option>
      </Select>
      <p v-else class="pb-2 text-sm text-muted-foreground">{{ $t('settings.llmProfiles.none') }}</p>
      <div v-if="mode === null" class="flex gap-1.5">
        <Button
          size="sm"
          variant="outline"
          :disabled="busy || saving || full"
          :title="full ? $t('settings.llmProfiles.full') : ''"
          @click="startSave"
        >
          {{ $t('settings.llmProfiles.saveAs') }}
        </Button>
        <Button
          v-if="activeProfile"
          size="icon"
          variant="ghost"
          class="h-8 w-8"
          :disabled="busy || saving"
          :aria-label="$t('settings.llmProfiles.rename')"
          :title="$t('settings.llmProfiles.rename')"
          @click="startRename"
        >
          <Pencil class="size-4" aria-hidden="true" />
        </Button>
        <Button
          v-if="activeProfile"
          size="icon"
          variant="ghost"
          class="h-8 w-8"
          :disabled="busy || saving"
          :aria-label="$t('settings.llmProfiles.remove')"
          :title="$t('settings.llmProfiles.remove')"
          @click="remove"
        >
          <Trash2 class="size-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
    <form v-if="mode !== null" class="flex items-center gap-2" @submit.prevent="submit">
      <Input
        v-model="name"
        maxlength="40"
        class="max-w-xs"
        :aria-label="$t('settings.llmProfiles.name')"
        :placeholder="$t('settings.llmProfiles.namePlaceholder')"
        autofocus
      />
      <Button size="icon" type="submit" class="h-9 w-9" :disabled="busy || !name.trim()" :aria-label="$t('common.save')">
        <Check class="size-4" aria-hidden="true" />
      </Button>
      <Button
        size="icon"
        type="button"
        variant="ghost"
        class="h-9 w-9"
        :aria-label="$t('common.cancel')"
        @click="mode = null"
      >
        <X class="size-4" aria-hidden="true" />
      </Button>
    </form>
    <p class="text-[11px] text-muted-foreground">{{ $t('settings.llmProfiles.hint') }}</p>
  </div>
</template>
