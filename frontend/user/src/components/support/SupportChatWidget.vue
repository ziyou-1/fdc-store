<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Headset, Loader2, X } from 'lucide-vue-next'
import { useSupportChat } from '../../composables/useSupportChat'
import { useToast } from '../../composables/useToast'
import SupportMessageList from './SupportMessageList.vue'
import SupportComposer from './SupportComposer.vue'
import SupportQuickReplies from './SupportQuickReplies.vue'
import type { SupportQuickReplyData } from '../../api/types'

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

const handlePickQuickReply = (reply: SupportQuickReplyData) => {
  void chat.sendText(reply.title)
}

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
      class="fixed right-4 z-50 flex w-[calc(100vw-2rem)] max-w-sm flex-col overflow-hidden rounded-2xl border border-border bg-card shadow-2xl"
      :class="panelPosition"
      style="height: min(32rem, calc(100vh - 8rem))"
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

      <SupportMessageList :messages="messages" :loading="loading" />

      <SupportQuickReplies :replies="quickReplies" @pick="handlePickQuickReply" />

      <SupportComposer
        :sending="sending"
        :uploading="uploading"
        @send="handleSend"
        @attach="handleAttach"
        @invalid="handleInvalid"
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
