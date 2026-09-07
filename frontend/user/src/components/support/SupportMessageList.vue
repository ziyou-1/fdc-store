<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Loader2, MessageCircle } from 'lucide-vue-next'
import { getImageUrl } from '../../utils/image'
import type { SupportMessageData } from '../../api/types'

const props = defineProps<{
  messages: SupportMessageData[]
  loading: boolean
}>()

const { t } = useI18n()
const scrollRef = ref<HTMLElement | null>(null)

// 贴底后再滚：新消息渲染完才能拿到正确的 scrollHeight
const scrollToBottom = async () => {
  await nextTick()
  const el = scrollRef.value
  if (el) el.scrollTop = el.scrollHeight
}

watch(() => props.messages.length, scrollToBottom)
watch(() => props.loading, (val) => { if (!val) void scrollToBottom() })

const formatTime = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

defineExpose({ scrollToBottom })
</script>

<template>
  <div ref="scrollRef" class="flex-1 space-y-3 overflow-y-auto px-4 py-3">
    <div v-if="loading" class="flex h-full items-center justify-center">
      <Loader2 class="h-5 w-5 animate-spin text-muted-foreground" aria-hidden="true" />
    </div>

    <div
      v-else-if="messages.length === 0"
      class="flex h-full flex-col items-center justify-center gap-2 text-center text-sm text-muted-foreground"
    >
      <MessageCircle class="h-8 w-8 opacity-50" :stroke-width="1.5" aria-hidden="true" />
      <p>{{ t('support.empty') }}</p>
    </div>

    <template v-else>
      <div
        v-for="message in messages"
        :key="message.id"
        class="flex"
        :class="message.sender_type === 'user' ? 'justify-end' : 'justify-start'"
      >
        <div class="max-w-[78%] space-y-1">
          <div
            class="rounded-2xl px-3 py-2 text-sm break-words"
            :class="message.sender_type === 'user'
              ? 'bg-primary text-primary-foreground rounded-br-sm'
              : 'bg-muted text-foreground rounded-bl-sm'"
          >
            <p v-if="message.content_type === 'text'" class="whitespace-pre-wrap">
              {{ message.content }}
            </p>

            <a
              v-else-if="message.content_type === 'image'"
              :href="getImageUrl(message.media_url)"
              target="_blank"
              rel="noopener noreferrer"
              class="block"
            >
              <img
                :src="getImageUrl(message.media_url)"
                :alt="t('support.imageAlt')"
                class="max-h-56 rounded-lg object-contain"
                loading="lazy"
              />
            </a>

            <video
              v-else-if="message.content_type === 'video'"
              :src="getImageUrl(message.media_url)"
              controls
              preload="metadata"
              class="max-h-56 w-full rounded-lg"
            />
          </div>
          <p
            class="px-1 text-[11px] text-muted-foreground"
            :class="message.sender_type === 'user' ? 'text-right' : 'text-left'"
          >
            {{ message.sender_type === 'user' ? t('support.you') : t('support.agent') }}
            · {{ formatTime(message.created_at) }}
          </p>
        </div>
      </div>
    </template>
  </div>
</template>
