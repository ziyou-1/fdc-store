<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy, CornerUpLeft, Loader2, MessageCircle, Undo2 } from 'lucide-vue-next'
import { getImageUrl } from '../../utils/image'
import { copyText } from '../../utils/clipboard'
import { canRevokeMessage, isRevoked, quoteSummary } from '../../utils/supportMessage'
import type { SupportMessageData } from '../../api/types'

const props = defineProps<{
  messages: SupportMessageData[]
  loading: boolean
  // now 由父组件的轮询推进，过了 2 分钟的撤回按钮会自己消失。
  now: number
  revoking: boolean
}>()

const emit = defineEmits<{
  (e: 'reply', message: SupportMessageData): void
  (e: 'revoke', message: SupportMessageData): void
  (e: 'notify', message: string): void
}>()

const { t } = useI18n()
const scrollRef = ref<HTMLElement | null>(null)

// 展开操作条的那条消息。存 id 而不是对象：轮询每 5 秒整体替换 messages 数组，
// 存对象引用的话操作条会被刷掉。
const activeID = ref<number | null>(null)

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

// 引用块要按 ID 反查原消息。只存 ID 不冗余快照：被引用的消息一旦撤回，
// 引用块必须跟着变成「已撤回」，否则原文会从引用里漏出来。
const messageByID = computed(() => {
  const map = new Map<number, SupportMessageData>()
  props.messages.forEach((item) => map.set(item.id, item))
  return map
})

const quotedMessage = (message: SupportMessageData) =>
  message.reply_to_id ? messageByID.value.get(message.reply_to_id) ?? null : null

const senderLabel = (message: SupportMessageData) =>
  message.sender_type === 'user' ? t('support.you') : t('support.agent')

const quotedSummary = (message: SupportMessageData) =>
  quoteSummary(message, {
    revoked: t('support.revoked'),
    image: t('support.quoteImage'),
    video: t('support.quoteVideo'),
  })

// 撤回按钮只对买家自己 2 分钟内发的消息出现，判定与后端 domain.CanRevokeBy 对齐。
const canRevoke = (message: SupportMessageData) => canRevokeMessage(message, props.now, 'user')

// 点气泡展开/收起操作条。
//
// 这里不能用后台那套 opacity-0 group-hover:opacity-100：Tailwind v4 默认把
// hover: / group-hover: 编译进 @media(hover:hover)，纯触屏手机上这条规则
// 根本不匹配，按钮会永远透明、永远按不到。而买家端主要就是手机在用。
const toggleActions = (message: SupportMessageData) => {
  if (isRevoked(message)) return
  activeID.value = activeID.value === message.id ? null : message.id
}

const closeActions = () => {
  activeID.value = null
}

const handleReply = (message: SupportMessageData) => {
  closeActions()
  emit('reply', message)
}

const handleRevoke = (message: SupportMessageData) => {
  closeActions()
  emit('revoke', message)
}

const handleCopy = async (message: SupportMessageData) => {
  // 图片/视频没有文字可复制，退而复制它的地址。
  const text = message.content_type === 'text' ? message.content : getImageUrl(message.media_url)
  if (!text) return
  closeActions()
  try {
    await copyText(text)
    emit('notify', t('support.copied'))
  } catch {
    emit('notify', t('support.copyFailed'))
  }
}

defineExpose({ scrollToBottom, closeActions })
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
          <!-- 客服撤回后只留一行灰字，内容不再露出来 -->
          <div
            v-if="isRevoked(message)"
            class="rounded-2xl border border-dashed border-border px-3 py-2 text-xs text-muted-foreground"
          >
            {{ message.sender_type === 'user' ? t('support.revokedByMe') : t('support.revoked') }}
          </div>
          <div
            v-else
            class="cursor-pointer rounded-2xl px-3 py-2 text-sm break-words"
            :class="message.sender_type === 'user'
              ? 'bg-primary text-primary-foreground rounded-br-sm'
              : 'bg-muted text-foreground rounded-bl-sm'"
            role="button"
            tabindex="0"
            :aria-label="t('support.actions')"
            :aria-expanded="activeID === message.id"
            @click="toggleActions(message)"
            @keydown.enter.prevent="toggleActions(message)"
            @keydown.space.prevent="toggleActions(message)"
          >
            <div
              v-if="quotedMessage(message)"
              class="mb-1.5 rounded-lg border-l-2 px-2 py-1 text-xs opacity-80"
              :class="message.sender_type === 'user'
                ? 'border-primary-foreground/60 bg-primary-foreground/10'
                : 'border-border bg-background/60'"
            >
              <p class="font-medium">{{ senderLabel(quotedMessage(message)!) }}</p>
              <p class="line-clamp-2 whitespace-pre-wrap break-words">
                {{ quotedSummary(quotedMessage(message)!) }}
              </p>
            </div>

            <p v-if="message.content_type === 'text'" class="whitespace-pre-wrap">
              {{ message.content }}
            </p>

            <!-- 媒体自己消费点击（看大图 / 播放），不要连带展开操作条 -->
            <a
              v-else-if="message.content_type === 'image'"
              :href="getImageUrl(message.media_url)"
              target="_blank"
              rel="noopener noreferrer"
              class="block"
              @click.stop
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
              @click.stop
            />
          </div>

          <!-- 操作条：点气泡展开。手机上不能靠 hover，见 toggleActions 的注释 -->
          <div
            v-if="activeID === message.id && !isRevoked(message)"
            class="flex items-center gap-1"
            :class="message.sender_type === 'user' ? 'justify-end' : 'justify-start'"
          >
            <button
              type="button"
              class="flex items-center gap-1 rounded-md border border-border bg-card px-2 py-1 text-xs text-muted-foreground active:bg-accent"
              @click="handleReply(message)"
            >
              <CornerUpLeft class="h-3.5 w-3.5" aria-hidden="true" />
              {{ t('support.reply') }}
            </button>
            <button
              type="button"
              class="flex items-center gap-1 rounded-md border border-border bg-card px-2 py-1 text-xs text-muted-foreground active:bg-accent"
              @click="handleCopy(message)"
            >
              <Copy class="h-3.5 w-3.5" aria-hidden="true" />
              {{ t('support.copy') }}
            </button>
            <button
              v-if="canRevoke(message)"
              type="button"
              class="flex items-center gap-1 rounded-md border border-border bg-card px-2 py-1 text-xs text-muted-foreground active:bg-accent disabled:opacity-50"
              :disabled="revoking"
              @click="handleRevoke(message)"
            >
              <Loader2 v-if="revoking" class="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
              <Undo2 v-else class="h-3.5 w-3.5" aria-hidden="true" />
              {{ t('support.revoke') }}
            </button>
          </div>

          <p
            class="px-1 text-[11px] text-muted-foreground"
            :class="message.sender_type === 'user' ? 'text-right' : 'text-left'"
          >
            {{ senderLabel(message) }}
            · {{ formatTime(message.created_at) }}
          </p>
        </div>
      </div>
    </template>
  </div>
</template>
