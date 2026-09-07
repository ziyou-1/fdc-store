import { computed, ref } from 'vue'
import { supportAPI } from '../api'
import type {
    SupportConversationData,
    SupportMessageData,
    SupportQuickReplyData,
} from '../api/types'

// 轮询间隔：展开时贴近实时，收起时只为了刷新未读角标，放慢以省请求。
const POLL_INTERVAL_OPEN = 5000
const POLL_INTERVAL_IDLE = 30000

// 模块级单例：浮窗在 App.vue 全局挂载一次，跨路由切换要保留会话与已读状态。
// 与 useToast 的写法一致。
const isOpen = ref(false)
const loading = ref(false)
const sending = ref(false)
const uploading = ref(false)
const errorMessage = ref('')
const conversation = ref<SupportConversationData | null>(null)
const messages = ref<SupportMessageData[]>([])
const quickReplies = ref<SupportQuickReplyData[]>([])
const quickRepliesLoaded = ref(false)
const initialized = ref(false)

let pollTimer: ReturnType<typeof setInterval> | null = null
let pollDelay = 0

const unreadCount = computed(() => conversation.value?.unread_for_user ?? 0)
const isClosed = computed(() => conversation.value?.status === 'closed')
const lastMessageID = computed(() => {
    const list = messages.value
    const last = list.length > 0 ? list[list.length - 1] : undefined
    return last?.id ?? 0
})

// 按 id 去重后追加：发送成功后本地先插入，随后轮询会再返回同一条。
function appendMessages(incoming: SupportMessageData[]) {
    if (!incoming || incoming.length === 0) return
    const seen = new Set(messages.value.map((m) => m.id))
    const fresh = incoming.filter((m) => !seen.has(m.id))
    if (fresh.length === 0) return
    messages.value = [...messages.value, ...fresh].sort((a, b) => a.id - b.id)
}

async function loadInitial() {
    loading.value = true
    errorMessage.value = ''
    try {
        const res = await supportAPI.messages({ page: 1, page_size: 50 })
        const data = res.data.data
        conversation.value = data.conversation
        messages.value = [...(data.messages ?? [])].sort((a, b) => a.id - b.id)
        initialized.value = true
    } catch (err: any) {
        errorMessage.value = err?.message ?? ''
    } finally {
        loading.value = false
    }
}

// 轮询失败必须吞掉：网络抖动不该弹 toast，也不该把已加载的消息清空。
async function poll() {
    try {
        if (isOpen.value) {
            const res = await supportAPI.messages({ after_id: lastMessageID.value })
            const data = res.data.data
            conversation.value = data.conversation
            appendMessages(data.messages ?? [])
            // 展开状态下收到新消息即视为已读
            if ((data.messages?.length ?? 0) > 0) {
                await markRead()
            }
        } else {
            const res = await supportAPI.conversation()
            conversation.value = res.data.data
        }
    } catch {
        // 静默：轮询失败不打扰用户
    }
}

function stopPolling() {
    if (pollTimer !== null) {
        clearInterval(pollTimer)
        pollTimer = null
        pollDelay = 0
    }
}

// 间隔随展开状态变化，需要重建 interval。相同间隔时不重建，避免每次调用都重置计时。
function startPolling() {
    const desired = isOpen.value ? POLL_INTERVAL_OPEN : POLL_INTERVAL_IDLE
    if (pollTimer !== null && pollDelay === desired) return
    stopPolling()
    pollDelay = desired
    pollTimer = setInterval(poll, desired)
}

async function markRead() {
    try {
        await supportAPI.markRead()
        if (conversation.value) {
            conversation.value = { ...conversation.value, unread_for_user: 0 }
        }
    } catch {
        // 已读失败不影响会话可用性
    }
}

async function loadQuickReplies() {
    if (quickRepliesLoaded.value) return
    try {
        const res = await supportAPI.quickReplies()
        quickReplies.value = res.data.data ?? []
        quickRepliesLoaded.value = true
    } catch {
        // 常见问题拉不到不阻塞聊天
    }
}

async function open() {
    isOpen.value = true
    if (!initialized.value) {
        await loadInitial()
    }
    void loadQuickReplies()
    void markRead()
    startPolling()
}

function close() {
    isOpen.value = false
    startPolling()
}

async function toggle() {
    if (isOpen.value) {
        close()
    } else {
        await open()
    }
}

async function sendText(content: string) {
    const trimmed = content.trim()
    if (!trimmed || sending.value) return false
    sending.value = true
    errorMessage.value = ''
    try {
        const res = await supportAPI.send({ content: trimmed })
        appendMessages([res.data.data])
        // 买家发言会重开已关闭的会话，本地状态同步跟上
        if (conversation.value) {
            conversation.value = { ...conversation.value, status: 'open' }
        }
        // 常见问题命中后服务端会立刻追加客服消息，这里马上拉一次，不等 5s 轮询
        await poll()
        return true
    } catch (err: any) {
        errorMessage.value = err?.message ?? ''
        return false
    } finally {
        sending.value = false
    }
}

async function sendAttachment(file: File) {
    if (uploading.value) return false
    uploading.value = true
    errorMessage.value = ''
    try {
        const uploaded = await supportAPI.uploadAttachment(file)
        const { media_url, content_type } = uploaded.data.data
        const res = await supportAPI.send({ content_type, media_url })
        appendMessages([res.data.data])
        if (conversation.value) {
            conversation.value = { ...conversation.value, status: 'open' }
        }
        return true
    } catch (err: any) {
        errorMessage.value = err?.message ?? ''
        return false
    } finally {
        uploading.value = false
    }
}

// 登出时调用：清空单例状态，避免下一个账号看到上一个账号的消息。
function reset() {
    stopPolling()
    isOpen.value = false
    initialized.value = false
    quickRepliesLoaded.value = false
    conversation.value = null
    messages.value = []
    quickReplies.value = []
    errorMessage.value = ''
}

export function useSupportChat() {
    return {
        isOpen,
        loading,
        sending,
        uploading,
        errorMessage,
        conversation,
        messages,
        quickReplies,
        unreadCount,
        isClosed,
        open,
        close,
        toggle,
        sendText,
        sendAttachment,
        markRead,
        startPolling,
        stopPolling,
        reset,
        poll,
    }
}
