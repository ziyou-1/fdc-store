<template>
  <Teleport to="body">
    <div
      v-if="open && blocks.length"
      class="fixed inset-0 z-[80] flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      :aria-label="t('orderDetail.instructionsTitle')"
    >
      <div class="absolute inset-0 bg-black/55" @click="close" />
      <div class="relative z-10 flex max-h-[86vh] w-full max-w-lg flex-col overflow-hidden rounded-2xl border bg-card text-card-foreground shadow-lg">
        <div class="flex items-center justify-between gap-3 border-b px-5 py-4">
          <h2 class="text-lg font-semibold">{{ t('orderDetail.instructionsTitle') }}</h2>
          <button
            type="button"
            class="inline-flex h-8 w-8 items-center justify-center rounded-full text-muted-foreground hover:bg-secondary hover:text-foreground"
            :aria-label="t('announcement.close')"
            @click="close"
          >
            <X class="h-4 w-4" :stroke-width="2" />
          </button>
        </div>
        <div class="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 py-4">
          <div v-for="(block, index) in blocks" :key="`${block.title}-${index}`">
            <div v-if="block.title" class="mb-2 text-sm font-semibold text-foreground">{{ block.title }}</div>
            <div
              class="prose prose-sm max-w-none break-words dark:prose-invert [&_img]:max-w-full [&_img]:rounded-lg [&_video]:max-w-full [&_video]:rounded-lg"
              v-html="block.html"
            />
          </div>
        </div>
        <div class="border-t px-5 py-3">
          <Button class="w-full sm:w-auto" @click="close">{{ t('announcement.close') }}</Button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { X } from 'lucide-vue-next'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import type { ContentTutorialBlock } from '../../utils/contentTutorial'

defineProps<{
  open: boolean
  blocks: ContentTutorialBlock[]
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
}>()

const { t } = useI18n()

const close = () => emit('update:open', false)
</script>
