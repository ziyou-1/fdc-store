<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Headset, Loader2, X } from 'lucide-vue-next'
import { useSupportChat } from '../../composables/useSupportChat'
import { useToast } from '../../composables/useToast'
import SupportMessageList from './SupportMessageList.vue'
import SupportComposer from './SupportComposer.vue'
import SupportQuickReplies from './SupportQuickReplies.vue'
import type { SupportMessageData, SupportQuickReplyData } from '../../api/types'

const props = withDefaults(
  defineProps<{
    // classic 模板底部有 MobileBottomNav（main 上是 pb-14），浮球要抬高避免遮挡；
    // vault 模板没有底部导航，可以贴底。
    offsetBottomMobile?: boolean
  }>(),
  { offsetBottomMobile: true },
)

const { t } = useI18n()
const { toast } = useToast()
const chat = useSupportChat()

const {
  isOpen, loading, sending, uploading, errorMessage,
  messages, quickReplies, unreadCount, isClosed,
  replyTo, revoking, nowTick,
} = chat

const badgeText = computed(() => (unreadCount.value > 99 ? '99+' : String(unreadCount.value)))

const panelPosition = computed(() =>
  props.offsetBottomMobile ? 'bottom-20 sm:bottom-24' : 'bottom-6',
)
const ballPosition = computed(() =>
  props.offsetBottomMobile ? 'bottom-20 lg:bottom-6' : 'bottom-6',
)

// 错误统一走 toast，面板内不再重复渲染错误块
watch(errorMessage, (msg) => {
  if (msg) {
    toast.error(msg)
    errorMessage.value = ''
  }
})

const handleSend = async (content: string) => {
  await chat.sendText(content)
}

const handleAttach = async (file: File) => {
  await chat.sendAttachment(file)
}

const handleInvalid = (message: string) => {
  toast.error(message)
}

const handleReply = (message: SupportMessageData) => {
  chat.setReplyTo(message)
}

const handleCancelReply = () => {
  chat.setReplyTo(null)
}

const handleRevoke = async (message: SupportMessageData) => {
  // 失败文案由服务端给（超时/越权），走 errorMessage 那条既有的 toast 通路
  await chat.revokeMessage(message.id)
}

// 消息列表里复制成功/失败的提示
const handleNotify = (message: string) => {
  toast.success(message)
}

const handlePickQuickReply = (reply: SupportQuickReplyData) => {
  void chat.sendText(reply.title)
}

// 面板收起时把展开着的操作条一并收掉，免得下次打开还挂在那里
const messageListRef = ref<{ closeActions: () => void } | null>(null)
watch(isOpen, (open) => {
  if (!open) messageListRef.value?.closeActions()
})

// 组件全局常驻，卸载时必须停轮询
onBeforeUnmount(() => {
  chat.stopPolling()
})

// 挂载即开始低频轮询，让未读角标在用户没点开时也能亮
chat.startPolling()
void chat.poll()
</script>

<template>
  <!-- 面板 -->
  <Transition name="support-panel">
    <section
      v-if="isOpen"
      class="support-panel fixed right-4 z-50 flex w-[calc(100vw-2rem)] max-w-sm flex-col overflow-hidden rounded-2xl border border-border bg-card shadow-2xl"
      :class="panelPosition"
      role="dialog"
      :aria-label="t('support.title')"
    >
      <header class="flex items-center justify-between border-b border-border px-4 py-3">
        <div class="flex items-center gap-2">
          <Headset class="h-4 w-4 text-primary" aria-hidden="true" />
          <h2 class="text-sm font-semibold">{{ t('support.title') }}</h2>
        </div>
        <button
          type="button"
          class="flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
          :aria-label="t('support.close')"
          @click="chat.close()"
        >
          <X class="h-4 w-4" aria-hidden="true" />
        </button>
      </header>

      <p v-if="isClosed" class="border-b border-border bg-muted/50 px-4 py-2 text-xs text-muted-foreground">
        {{ t('support.closedHint') }}
      </p>

      <SupportMessageList
        ref="messageListRef"
        :messages="messages"
        :loading="loading"
        :now="nowTick"
        :revoking="revoking"
        @reply="handleReply"
        @revoke="handleRevoke"
        @notify="handleNotify"
      />

      <SupportQuickReplies :replies="quickReplies" @pick="handlePickQuickReply" />

      <SupportComposer
        :sending="sending"
        :uploading="uploading"
        :reply-to="replyTo"
        @send="handleSend"
        @attach="handleAttach"
        @invalid="handleInvalid"
        @cancel-reply="handleCancelReply"
      />
    </section>
  </Transition>

  <!-- 浮球 -->
  <button
    v-if="!isOpen"
    type="button"
    class="fixed right-4 z-50 flex h-12 w-12 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-lg transition-transform hover:scale-105"
    :class="ballPosition"
    :aria-label="t('support.title')"
    @click="chat.toggle()"
  >
    <Loader2 v-if="loading" class="h-5 w-5 animate-spin" aria-hidden="true" />
    <Headset v-else class="h-5 w-5" aria-hidden="true" />
    <span
      v-if="unreadCount > 0"
      class="absolute -right-0.5 -top-0.5 flex h-5 min-w-5 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-medium text-destructive-foreground"
    >
      {{ badgeText }}
    </span>
  </button>
</template>

<style scoped>
/*
 * 面板高度。
 *
 * iOS Safari（以及微信/QQ 等内置浏览器）的 100vh 取的是「工具栏收起时」的大视口，
 * 比用户真正看得见的那块高 100~150px。面板照着它算高度，就会顶出可见区域——
 * iPhone SE 上实测面板 512px、可用空间只有 473px，顶端被切掉 39px。
 *
 * svh 取的是「工具栏展开时」的小视口，是永远看得见的那部分，面板不会超出。
 * 不用 dvh：dvh 随工具栏收放连续变化，滚动时面板高度会跟着抖。
 *
 * 用 @supports 而不是写两遍 height：构建链里的 Lightning CSS 会按 browserslist
 * 判断要不要保留重复声明，@supports 是显式的、不会被优化掉。
 */
.support-panel {
  height: min(32rem, calc(100vh - 8rem));
}

@supports (height: 100svh) {
  .support-panel {
    height: min(32rem, calc(100svh - 8rem));
  }
}

.support-panel-enter-active,
.support-panel-leave-active {
  transition: opacity 180ms ease, transform 180ms ease;
}

.support-panel-enter-from,
.support-panel-leave-to {
  opacity: 0;
  transform: translateY(8px) scale(0.98);
}
</style>
