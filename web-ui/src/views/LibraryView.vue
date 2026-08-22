<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { toast } from 'vue-sonner'
import { useI18n } from 'vue-i18n'
import { Trash2 } from '@lucide/vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { api } from '@/api'
import type { TextSummary } from '@/types'

const router = useRouter()
const { t } = useI18n()

const texts = ref<TextSummary[]>([])
const title = ref('')
const body = ref('')
const kind = ref('paste')
const busy = ref(false)

async function load() {
  texts.value = (await api.texts()).texts
}

async function add() {
  if (!body.value.trim()) return
  busy.value = true
  try {
    const res = await api.createText({ title: title.value, body: body.value, kind: kind.value })
    toast.success(t('library.added', { title: res.title }))
    title.value = ''
    body.value = ''
    await load()
    router.push(`/read/${res.id}`)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('errors.generic'))
  } finally {
    busy.value = false
  }
}

async function remove(id: number) {
  await api.deleteText(id)
  await load()
}

onMounted(load)

const pct = (n: number) => `${Math.round(n * 100)}%`
</script>

<template>
  <div class="mx-auto flex w-full max-w-3xl flex-col gap-6">
    <h1 class="text-2xl font-semibold tracking-tight">{{ $t('nav.library') }}</h1>

    <Card>
      <CardHeader>
        <CardTitle class="text-base">{{ $t('library.addTitle') }}</CardTitle>
      </CardHeader>
      <CardContent class="flex flex-col gap-3">
        <div class="grid gap-3 sm:grid-cols-[1fr_10rem]">
          <div class="flex flex-col gap-1.5">
            <Label for="t">{{ $t('library.title') }}</Label>
            <Input id="t" v-model="title" :placeholder="$t('library.titleHint')" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="k">{{ $t('library.kind') }}</Label>
            <Select id="k" v-model="kind">
              <option value="paste">{{ $t('library.kinds.paste') }}</option>
              <option value="html">{{ $t('library.kinds.html') }}</option>
              <option value="srt">{{ $t('library.kinds.srt') }}</option>
            </Select>
          </div>
        </div>
        <div class="flex flex-col gap-1.5">
          <Label for="b">{{ $t('library.text') }}</Label>
          <textarea
            id="b"
            v-model="body"
            rows="6"
            lang="hu"
            class="w-full rounded-md border border-input bg-transparent p-3 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            :placeholder="$t('library.textHint')"
          />
        </div>
        <Button class="self-start" :disabled="busy || !body.trim()" @click="add">
          {{ busy ? $t('common.saving') : $t('library.add') }}
        </Button>
      </CardContent>
    </Card>

    <div v-if="texts.length" class="flex flex-col gap-2">
      <div
        v-for="tx in texts"
        :key="tx.id"
        class="flex items-center gap-3 rounded-md border border-border p-3"
      >
        <RouterLink :to="`/read/${tx.id}`" class="min-w-0 flex-1">
          <p class="truncate text-sm font-medium">{{ tx.title }}</p>
          <p class="text-xs text-muted-foreground">
            {{ tx.wordCount }} {{ $t('library.words') }} · {{ $t('reader.lemmaCoverage') }}
            {{ pct(tx.lemmaCoverage) }} · {{ $t('reader.elementCoverage') }}
            {{ pct(tx.elementCoverage) }}
          </p>
        </RouterLink>
        <Button
          variant="ghost"
          size="icon"
          :aria-label="$t('library.remove')"
          @click="remove(tx.id)"
        >
          <Trash2 class="size-4" />
        </Button>
      </div>
    </div>
    <p v-else class="text-sm text-muted-foreground">{{ $t('library.empty') }}</p>
  </div>
</template>
