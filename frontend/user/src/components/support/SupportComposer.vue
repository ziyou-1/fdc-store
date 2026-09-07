<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Loader2, Paperclip, SendHorizontal } from 'lucide-vue-next'

const props = defineProps<{
  sending: boolean
  uploading: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'send', content: string): void
  (e: 'attach', file: File): void
  (e: 'invalid', message: string): void
}>()

const { t } = useI18n()

// 与后端 upload.support 策略保持一致（config.go: max_size 52428800 + 8 种扩展名）。
// 前端先拦一道，避免用户等上传完才被拒。
const MAX_ATTACHMENT_BYTES = 52428800
const ACCEPT = '.jpg,.jpeg,.png,.gif,.webp,.mp4,.webm,.mov'

const draft = ref('')
const fileInput = ref<HTMLInputElement | null>(null)

const busy = computed(() => props.sending || props.uploading || props.disabled === true)
const canSend = computed(() => draft.value.trim().length > 0 && !busy.value)

const submit = () => {
  if (!canSend.value) return
  emit('send', draft.value)
  draft.value = ''
}

// Enter 发送，Shift+Enter 换行；输入法组合期间不触发（isComposing）。
const handleKeydown = (event: KeyboardEvent) => {
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return
  event.preventDefault()
  submit()
}

const pickFile = () => {
  if (busy.value) return
  fileInput.value?.click()
}

const handleFileChange = (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  // 无论是否通过校验都要清空，否则连续选同一个文件不会再触发 change
  input.value = ''
  if (!file) return
  if (file.size > MAX_ATTACHMENT_BYTES) {
    emit('invalid', t('support.errors.attachmentTooLarge', { size: 50 }))
    return
  }
  emit('attach', file)
}

// 供父组件把常见问题填进输入框
const fill = (content: string) => {
  draft.value = content
}

defineExpose({ fill })
</script>

<template>
  <div class="border-t border-border p-3">
    <div class="flex items-end gap-2">
      <button
        type="button"
        class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground disabled:pointer-events-none disabled:opacity-50"
        :disabled="busy"
        :aria-label="t('support.attach')"
        @click="pickFile"
      >
        <Loader2 v-if="uploading" class="h-4 w-4 animate-spin" aria-hidden="true" />
        <Paperclip v-else class="h-4 w-4" aria-hidden="true" />
      </button>

      <textarea
        v-model="draft"
        rows="1"
        :placeholder="t('support.placeholder')"
        :disabled="busy"
        class="max-h-28 min-h-9 flex-1 resize-none rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:opacity-50"
        @keydown="handleKeydown"
      />

      <button
        type="button"
        class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md bg-primary text-primary-foreground shadow transition-colors hover:bg-primary/90 disabled:pointer-events-none disabled:opacity-50"
        :disabled="!canSend"
        :aria-label="t('support.send')"
        @click="submit"
      >
        <Loader2 v-if="sending" class="h-4 w-4 animate-spin" aria-hidden="true" />
        <SendHorizontal v-else class="h-4 w-4" aria-hidden="true" />
      </button>
    </div>

    <input
      ref="fileInput"
      type="file"
      :accept="ACCEPT"
      class="hidden"
      @change="handleFileChange"
    />
  </div>
</template>
