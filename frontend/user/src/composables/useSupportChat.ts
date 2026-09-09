import { computed, ref } from 'vue'
import { supportAPI } from '../api'
import { mergeRevocations, nextRevokedCursor } from '../utils/supportMessage'
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
// replyTo 是正在引用的那条消息，发送成功后清空。
const replyTo = ref<SupportMessageData | null>(null)
const revoking = ref(false)
// revokedCursor 是撤回的增量游标：服务端产出、这里原样带回，
// 因此不受两边时钟不一致影响，也不随离线时长衰减——挂机几小时回来照样对得上。
const revokedCursor = ref('')
// nowTick 由轮询推进，让过了 2 分钟的撤回按钮自己消失，不另开定时器。
const nowTick = ref(0)

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

// 撤回是状态变更不是新消息：它不产生新 id，上面那个「见过就跳过」的
// appendMessages 会把它整条丢掉。这才是「撤回后要刷新一下才看得到」的根因。
function applyRevocations(result: {
    revoked?: SupportMessageData[]
    revoked_cursor?: string | null
}) {
    messages.value = mergeRevocations(messages.value, result.revoked ?? [])
    revokedCursor.value = nextRevokedCursor(revokedCursor.value, result.revoked_cursor)
}

async function loadInitial() {
    loading.value = true
    errorMessage.value = ''
    try {
        const res = await supportAPI.messages({ page: 1, page_size: 50 })
        const data = res.data.data
        conversation.value = data.conversation
        messages.value = [...(data.messages ?? [])].sort((a, b) => a.id - b.id)
        // 首屏的消息本来就带着 revoked_at，这里只需把游标接上，
        // 之后的增量轮询才知道该从哪儿开始对账。
        revokedCursor.value = nextRevokedCursor(revokedCursor.value, data.revoked_cursor)
        nowTick.value = Date.now()
        initialized.value = true
    } catch (err: any) {
        errorMessage.value = err?.message ?? ''
    } finally {
        loading.value = false
    }
}

// 轮询失败必须吞掉：网络抖动不该弹 toast，也不该把已加载的消息清空。
async function poll() {
    // 顺带推进时间基准，撤回按钮到点会自己消失。
    nowTick.value = Date.now()
    try {
        if (isOpen.value) {
            const res = await supportAPI.messages({
                after_id: lastMessageID.value,
                revoked_after: revokedCursor.value || undefined,
            })
            const data = res.data.data
            conversation.value = data.conversation
            appendMessages(data.messages ?? [])
            applyRevocations(data)
            // 展开状态下收到新消息即视为已读。
            // 判定只看 messages，不能把 revoked 算进来：撤回会被反复回传，
            // 算进来就等于每 5 秒把买家还没看的消息标成已读。
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
    } else {
        // 收起期间只刷未读角标、不拉消息，这段时间的撤回没同步过。
        // 重新展开时立刻对一次账，不等 5 秒后的下一轮。
        void poll()
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
        const res = await supportAPI.send({
            content: trimmed,
            reply_to_id: replyTo.value?.id,
        })
        appendMessages([res.data.data])
        // 发送成功才清引用：失败时保留，否则买家得重新找回那条再引一次
        replyTo.value = null
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
        const res = await supportAPI.send({
            content_type,
            media_url,
            reply_to_id: replyTo.value?.id,
        })
        appendMessages([res.data.data])
        replyTo.value = null
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

function setReplyTo(message: SupportMessageData | null) {
    // 引用一条已撤回的消息没有意义：摘要只会显示「已撤回」
    if (message && message.revoked_at) return
    replyTo.value = message
}

// 撤回自己 2 分钟内发的消息。服务端才是判定方，这里失败就把文案交给
// errorMessage，走 Widget 里既有的 toast 通路。
async function revokeMessage(id: number) {
    if (!id || revoking.value) return false
    revoking.value = true
    errorMessage.value = ''
    try {
        const res = await supportAPI.revoke(id)
        const updated = res.data.data as SupportMessageData
        applyRevocations({ revoked: [updated], revoked_cursor: updated.revoked_at })
        if (replyTo.value?.id === id) replyTo.value = null
        return true
    } catch (err: any) {
        errorMessage.value = err?.message ?? ''
        return false
    } finally {
        revoking.value = false
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
    quickRepliesLoaded.value = false
    errorMessage.value = ''
    replyTo.value = null
    revoking.value = false
    revokedCursor.value = ''
    nowTick.value = 0
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
        replyTo,
        revoking,
        nowTick,
        setReplyTo,
        revokeMessage,
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
