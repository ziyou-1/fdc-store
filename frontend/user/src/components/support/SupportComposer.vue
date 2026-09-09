<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Loader2, Paperclip, SendHorizontal, X } from 'lucide-vue-next'
import { quoteSummary } from '../../utils/supportMessage'
import type { SupportMessageData } from '../../api/types'

const props = defineProps<{
  sending: boolean
  uploading: boolean
  disabled?: boolean
  // replyTo 由父组件持有：发送成功后才清，失败时草稿与引用都要留住。
  replyTo?: SupportMessageData | null
}>()

const emit = defineEmits<{
  (e: 'send', content: string): void
  (e: 'attach', file: File): void
  (e: 'invalid', message: string): void
  (e: 'cancel-reply'): void
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

const quotedLabel = computed(() => {
  const target = props.replyTo
  if (!target) return ''
  return target.sender_type === 'user' ? t('support.you') : t('support.agent')
})

const quotedText = computed(() => {
  const target = props.replyTo
  if (!target) return ''
  return quoteSummary(target, {
    revoked: t('support.revoked'),
    image: t('support.quoteImage'),
    video: t('support.quoteVideo'),
  })
})

// 供父组件把常见问题填进输入框
const fill = (content: string) => {
  draft.value = content
}

defineExpose({ fill })
</script>

<template>
  <div class="border-t border-border p-3">
    <!-- 正在引用哪条：发送成功后自动清空，也可以手动叉掉 -->
    <div
      v-if="replyTo"
      class="mb-2 flex items-start gap-2 rounded-lg border-l-2 border-primary bg-muted/60 px-2 py-1.5"
    >
      <div class="min-w-0 flex-1 text-xs">
        <p class="font-medium">{{ t('support.replyingTo', { name: quotedLabel }) }}</p>
        <p class="line-clamp-2 whitespace-pre-wrap break-words text-muted-foreground">
          {{ quotedText }}
        </p>
      </div>
      <button
        type="button"
        class="shrink-0 rounded-md p-1 text-muted-foreground"
        :aria-label="t('support.cancelReply')"
        @click="emit('cancel-reply')"
      >
        <X class="h-3.5 w-3.5" aria-hidden="true" />
      </button>
    </div>
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
