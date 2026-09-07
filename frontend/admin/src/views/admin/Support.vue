<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Loader2, Paperclip, Pencil, Plus, SendHorizontal, Trash2 } from 'lucide-vue-next'
import { adminAPI } from '@/api/admin'
import type {
  AdminSupportAttachmentResult,
  AdminSupportConversation,
  AdminSupportMessage,
  AdminSupportMessageListResult,
  AdminSupportQuickReply,
  AdminSupportUnreadCount,
} from '@/api/types'
import ListPagination from '@/components/ListPagination.vue'
import { Button } from '@/components/ui/button'
import { Dialog, DialogHeader, DialogScrollContent, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useAdminAuthStore } from '@/stores/auth'
import { confirmAction } from '@/utils/confirm'
import { formatDate, formatMoney } from '@/utils/format'
import { orderStatusClass, orderStatusLabel } from '@/utils/status'
import { getImageUrl } from '@/utils/image'
import { notifySuccess } from '@/utils/notify'

const STATUS_ALL = '__all__'
const LIST_POLL_MS = 10000
const MESSAGE_POLL_MS = 5000
const MAX_ATTACHMENT_BYTES = 50 * 1024 * 1024
const MAX_CONTENT_LENGTH = 4000
const ALLOWED_ATTACHMENT_TYPES = new Set([
  'image/jpeg',
  'image/png',
  'image/gif',
  'image/webp',
  'video/mp4',
  'video/webm',
  'video/quicktime',
])

const { t } = useI18n()
const authStore = useAdminAuthStore()

const loadingList = ref(true)
const loadingMessages = ref(false)
const sending = ref(false)
const uploading = ref(false)
const statusUpdating = ref(false)
const draft = ref('')
const formError = ref('')
const conversations = ref<AdminSupportConversation[]>([])
const messages = ref<AdminSupportMessage[]>([])
const selectedId = ref<number | null>(null)
const selected = ref<AdminSupportConversation | null>(null)
const unreadTotal = ref(0)
const pagination = ref({ page: 1, page_size: 20, total: 0, total_page: 1 })
const pageSizeOptions = [10, 20, 50]
const filters = reactive({
  status: STATUS_ALL,
  unreadOnly: false,
  userId: '',
})
const scrollRef = ref<HTMLElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const quickReplies = ref<AdminSupportQuickReply[]>([])
const showQuickReplyModal = ref(false)
const quickReplySubmitting = ref(false)
const quickReplyFormError = ref('')
const editingQuickReplyId = ref<number | null>(null)
const quickReplyForm = reactive({
  title: '',
  content: '',
  is_active: true,
  sort_order: 0,
})

let listTimer: ReturnType<typeof setInterval> | null = null
let messageTimer: ReturnType<typeof setInterval> | null = null

const canReply = computed(() => authStore.hasPermission('POST:/admin/support/conversations/:id/messages'))
const canStatus = computed(() => authStore.hasPermission('PUT:/admin/support/conversations/:id/status'))
const canUpload = computed(() => authStore.hasPermission('POST:/admin/support/attachments'))
const canOpenUser = computed(() => authStore.hasPermission('GET:/admin/users/:id'))
// 客服角色在 RBAC seed 里已带 GET:/admin/orders，
// 没有该权限的角色（如纯只读审计）不渲染侧栏而不是让它 403。
const canViewOrders = computed(() => authStore.hasPermission('GET:/admin/orders'))
const canOpenOrder = computed(() => authStore.hasPermission('GET:/admin/orders/:id'))
const canCreateQuickReply = computed(() => authStore.hasPermission('POST:/admin/support/quick-replies'))
const canUpdateQuickReply = computed(() => authStore.hasPermission('PUT:/admin/support/quick-replies/:id'))
const canDeleteQuickReply = computed(() => authStore.hasPermission('DELETE:/admin/support/quick-replies/:id'))
const canWriteQuickReply = computed(() => canCreateQuickReply.value || canUpdateQuickReply.value)
const composerBusy = computed(() => sending.value || uploading.value || !selectedId.value)
const isClosed = computed(() => selected.value?.status === 'closed')
const activeQuickReplies = computed(() =>
  quickReplies.value.filter((item) => item.is_active),
)

const asInt = (value: unknown) => {
  const n = Math.trunc(Number(value))
  return Number.isFinite(n) ? n : 0
}

const lastMessageID = () => {
  const last = messages.value[messages.value.length - 1]
  return last ? last.id : 0
}

const scrollToBottom = async () => {
  await nextTick()
  const el = scrollRef.value
  if (el) el.scrollTop = el.scrollHeight
}

const appendMessages = (incoming: AdminSupportMessage[]) => {
  if (incoming.length === 0) return
  const seen = new Set(messages.value.map((item) => item.id))
  const fresh = incoming.filter((item) => !seen.has(item.id))
  if (fresh.length === 0) return
  messages.value = [...messages.value, ...fresh].sort((a, b) => a.id - b.id)
  void scrollToBottom()
}

const applyConversation = (next: AdminSupportConversation) => {
  const currentSelected = selected.value
  if (currentSelected && currentSelected.id === next.id) {
    selected.value = {
      ...currentSelected,
      ...next,
      user_email: next.user_email || currentSelected.user_email,
    }
  }
  conversations.value = conversations.value.map((item) => {
    if (item.id !== next.id) return item
    return {
      ...item,
      ...next,
      user_email: next.user_email || item.user_email,
    }
  })
}

const fetchUnread = async (silent: boolean) => {
  try {
    const response = await adminAPI.getSupportUnreadCount(silent ? { silent: true } : undefined)
    const data = response.data.data as AdminSupportUnreadCount | undefined
    unreadTotal.value = Number(data?.unread_conversations ?? 0)
  } catch {
    if (!silent) unreadTotal.value = 0
  }
}

const fetchList = async (options: { silent?: boolean; page?: number } = {}) => {
  const silent = options.silent === true
  if (!silent) loadingList.value = true
  try {
    const userId = Number(filters.userId)
    const response = await adminAPI.getSupportConversations(
      {
        page: options.page ?? pagination.value.page,
        page_size: pagination.value.page_size,
        status: filters.status === STATUS_ALL ? undefined : filters.status,
        unread_only: filters.unreadOnly ? true : undefined,
        user_id: Number.isFinite(userId) && userId > 0 ? userId : undefined,
      },
      silent ? { silent: true } : undefined,
    )
    conversations.value = (response.data.data || []) as AdminSupportConversation[]
    pagination.value = response.data.pagination || pagination.value
    if (selectedId.value) {
      const row = conversations.value.find((item) => item.id === selectedId.value)
      if (row) selected.value = row
    }
    void fetchUnread(true)
  } catch {
    if (!silent) conversations.value = []
  } finally {
    if (!silent) loadingList.value = false
  }
}

const markRead = async (id: number) => {
  try {
    await adminAPI.markSupportRead(id, { silent: true })
    const currentSelected = selected.value
    if (currentSelected && currentSelected.id === id) {
      selected.value = { ...currentSelected, unread_for_agent: 0 }
    }
    conversations.value = conversations.value.map((item) =>
      item.id === id ? { ...item, unread_for_agent: 0 } : item,
    )
  } catch {
    // 已读失败不打断会话
  }
}

const loadMessages = async (id: number) => {
  loadingMessages.value = true
  formError.value = ''
  try {
    const response = await adminAPI.getSupportMessages(id, { page: 1, page_size: 50 })
    const data = response.data.data as AdminSupportMessageListResult
    messages.value = [...(data.messages ?? [])].sort((a, b) => a.id - b.id)
    if (data.conversation) applyConversation(data.conversation)
    await markRead(id)
    void scrollToBottom()
  } catch {
    messages.value = []
  } finally {
    loadingMessages.value = false
  }
}

const pollMessages = async () => {
  if (!selectedId.value) return
  try {
    const response = await adminAPI.getSupportMessages(
      selectedId.value,
      { after_id: lastMessageID() },
      { silent: true },
    )
    const data = response.data.data as AdminSupportMessageListResult
    if (data.conversation) applyConversation(data.conversation)
    const incoming = data.messages ?? []
    appendMessages(incoming)
    if (incoming.length > 0) await markRead(selectedId.value)
  } catch {
    // 轮询失败不打扰
  }
}

const stopPolling = () => {
  if (listTimer !== null) {
    clearInterval(listTimer)
    listTimer = null
  }
  if (messageTimer !== null) {
    clearInterval(messageTimer)
    messageTimer = null
  }
}

const startPolling = () => {
  stopPolling()
  listTimer = setInterval(() => {
    void fetchList({ silent: true })
  }, LIST_POLL_MS)
  messageTimer = setInterval(() => {
    void pollMessages()
  }, MESSAGE_POLL_MS)
}

// 订单上下文侧栏：点开会话即可见该买家最近的订单，
// 免去"请提供订单号"这一轮来回。
const ORDER_SIDEBAR_LIMIT = 8
const buyerOrders = ref<any[]>([])
const loadingOrders = ref(false)
const orderStatusText = (status?: string) => orderStatusLabel(t, status)

const loadBuyerOrders = async (userID: number) => {
  if (!canViewOrders.value || !userID) {
    buyerOrders.value = []
    return
  }
  loadingOrders.value = true
  try {
    const response = await adminAPI.getOrders({
      user_id: userID,
      page: 1,
      page_size: ORDER_SIDEBAR_LIMIT,
      silent: true,
    })
    buyerOrders.value = response.data.data || []
  } catch {
    // 侧栏是辅助信息，拉不到就留空，不打断客服回复
    buyerOrders.value = []
  } finally {
    loadingOrders.value = false
  }
}

const selectConversation = async (row: AdminSupportConversation) => {
  if (selectedId.value === row.id) return
  selectedId.value = row.id
  selected.value = row
  messages.value = []
  buyerOrders.value = []
  await Promise.all([loadMessages(row.id), loadBuyerOrders(row.user_id)])
}

const sendText = async () => {
  if (!canReply.value || !selectedId.value || sending.value) return
  const content = draft.value.trim()
  if (!content) return
  if (content.length > MAX_CONTENT_LENGTH) {
    formError.value = t('admin.support.errors.contentTooLong')
    return
  }
  sending.value = true
  formError.value = ''
  try {
    const response = await adminAPI.sendSupportMessage(selectedId.value, {
      content_type: 'text',
      content,
    })
    appendMessages([response.data.data as AdminSupportMessage])
    draft.value = ''
    void fetchList({ silent: true })
  } catch {
    // client 已提示
  } finally {
    sending.value = false
  }
}

const sendAttachment = async (file: File) => {
  if (!canReply.value || !canUpload.value || !selectedId.value || uploading.value) return
  if (!ALLOWED_ATTACHMENT_TYPES.has(file.type)) {
    formError.value = t('admin.support.errors.attachmentType')
    return
  }
  if (file.size > MAX_ATTACHMENT_BYTES) {
    formError.value = t('admin.support.errors.attachmentTooLarge')
    return
  }
  uploading.value = true
  formError.value = ''
  try {
    const uploaded = await adminAPI.uploadSupportAttachment(file)
    const data = uploaded.data.data as AdminSupportAttachmentResult | undefined
    if (!data?.media_url || !data.content_type) {
      formError.value = t('admin.support.errors.uploadEmpty')
      return
    }
    const response = await adminAPI.sendSupportMessage(selectedId.value, {
      content_type: data.content_type,
      media_url: data.media_url,
    })
    appendMessages([response.data.data as AdminSupportMessage])
    void fetchList({ silent: true })
  } catch {
    // client 已提示
  } finally {
    uploading.value = false
  }
}

const handleKeydown = (event: KeyboardEvent) => {
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return
  event.preventDefault()
  void sendText()
}

const handleFileChange = (event: Event) => {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (file) void sendAttachment(file)
}

const fillQuickReply = (reply: AdminSupportQuickReply) => {
  draft.value = reply.content.trim() || reply.title
}

const resetQuickReplyForm = () => {
  quickReplyForm.title = ''
  quickReplyForm.content = ''
  quickReplyForm.is_active = true
  quickReplyForm.sort_order = 0
  quickReplyFormError.value = ''
  editingQuickReplyId.value = null
}

const openCreateQuickReply = () => {
  resetQuickReplyForm()
  showQuickReplyModal.value = true
}

const openEditQuickReply = (reply: AdminSupportQuickReply) => {
  editingQuickReplyId.value = reply.id
  quickReplyForm.title = reply.title
  quickReplyForm.content = reply.content
  quickReplyForm.is_active = reply.is_active
  quickReplyForm.sort_order = reply.sort_order
  quickReplyFormError.value = ''
  showQuickReplyModal.value = true
}

const closeQuickReplyModal = () => {
  showQuickReplyModal.value = false
  resetQuickReplyForm()
}

const loadQuickReplies = async () => {
  if (!authStore.hasPermission('GET:/admin/support/quick-replies')) return
  try {
    const response = await adminAPI.getSupportQuickReplies({ silent: true })
    const list = (response.data.data || []) as AdminSupportQuickReply[]
    quickReplies.value = [...list].sort((a, b) => a.sort_order - b.sort_order || a.id - b.id)
  } catch {
    quickReplies.value = []
  }
}

const submitQuickReply = async () => {
  const title = quickReplyForm.title.trim()
  const content = quickReplyForm.content.trim()
  if (!title) {
    quickReplyFormError.value = t('admin.supportQuickReplies.errors.titleRequired')
    return
  }
  if (title.length > 120) {
    quickReplyFormError.value = t('admin.supportQuickReplies.errors.titleTooLong')
    return
  }
  if (content.length > 4000) {
    quickReplyFormError.value = t('admin.supportQuickReplies.errors.contentTooLong')
    return
  }
  if (editingQuickReplyId.value && !canUpdateQuickReply.value) return
  if (!editingQuickReplyId.value && !canCreateQuickReply.value) return

  quickReplySubmitting.value = true
  quickReplyFormError.value = ''
  try {
    const payload = {
      title,
      content,
      is_active: quickReplyForm.is_active,
      sort_order: asInt(quickReplyForm.sort_order),
    }
    if (editingQuickReplyId.value) {
      await adminAPI.updateSupportQuickReply(editingQuickReplyId.value, payload)
    } else {
      await adminAPI.createSupportQuickReply(payload)
    }
    notifySuccess(t('admin.supportQuickReplies.saveSuccess'))
    closeQuickReplyModal()
    await loadQuickReplies()
  } catch {
    // client 已提示
  } finally {
    quickReplySubmitting.value = false
  }
}

const removeQuickReply = async (reply: AdminSupportQuickReply) => {
  if (!canDeleteQuickReply.value) return
  const confirmed = await confirmAction({
    description: t('admin.supportQuickReplies.deleteConfirm', { title: reply.title }),
    confirmText: t('admin.common.delete'),
    variant: 'destructive',
  })
  if (!confirmed) return
  try {
    await adminAPI.deleteSupportQuickReply(reply.id)
    notifySuccess(t('admin.supportQuickReplies.deleteSuccess'))
    await loadQuickReplies()
  } catch {
    // client 已提示
  }
}

const toggleStatus = async () => {
  if (!canStatus.value || !selected.value || statusUpdating.value) return
  const next = selected.value.status === 'closed' ? 'open' : 'closed'
  statusUpdating.value = true
  try {
    const response = await adminAPI.updateSupportStatus(selected.value.id, { status: next })
    applyConversation({ ...selected.value, ...(response.data.data as AdminSupportConversation) })
    void fetchList({ silent: true })
  } catch {
    // client 已提示
  } finally {
    statusUpdating.value = false
  }
}

const changePage = (page: number) => {
  if (page < 1 || page > pagination.value.total_page) return
  void fetchList({ page })
}

const changePageSize = (size: number) => {
  if (size === pagination.value.page_size) return
  pagination.value.page_size = size
  void fetchList({ page: 1 })
}

const applyUserFilter = () => {
  void fetchList({ page: 1 })
}

const conversationTitle = (row: AdminSupportConversation) => {
  const email = (row.user_email || '').trim()
  if (email) return email
  return t('admin.support.userFallback', { id: row.user_id })
}

watch(
  () => [filters.status, filters.unreadOnly] as const,
  () => {
    void fetchList({ page: 1 })
  },
)

onMounted(() => {
  void fetchList({ page: 1 })
  void loadQuickReplies()
  startPolling()
})

onBeforeUnmount(stopPolling)
</script>

<template>
  <div class="space-y-4">
    <div>
      <h1 class="text-2xl font-semibold">{{ t('admin.support.title') }}</h1>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('admin.support.subtitle') }}</p>
      <p class="mt-1 text-xs text-muted-foreground">
        {{ t('admin.support.unread', { n: unreadTotal }) }}
      </p>
    </div>

    <div class="flex min-h-[640px] flex-col gap-4 lg:h-[calc(100vh-12rem)] lg:flex-row">
      <aside class="flex w-full flex-col rounded-xl border border-border bg-card shadow-sm lg:max-w-sm">
        <div class="space-y-3 border-b border-border p-4">
          <div>
            <Label class="mb-1.5 block text-xs text-muted-foreground">{{ t('admin.support.filters.status') }}</Label>
            <Select v-model="filters.status">
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem :value="STATUS_ALL">{{ t('admin.support.filters.statusAll') }}</SelectItem>
                <SelectItem value="open">{{ t('admin.support.filters.open') }}</SelectItem>
                <SelectItem value="closed">{{ t('admin.support.filters.closed') }}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="flex items-center justify-between gap-3">
            <Label class="text-sm">{{ t('admin.support.filters.unreadOnly') }}</Label>
            <Switch v-model="filters.unreadOnly" />
          </div>
          <div>
            <Label class="mb-1.5 block text-xs text-muted-foreground">{{ t('admin.support.filters.userId') }}</Label>
            <Input
              v-model="filters.userId"
              inputmode="numeric"
              @keydown.enter.prevent="applyUserFilter"
            />
          </div>
        </div>

        <div class="min-h-0 flex-1 overflow-y-auto">
          <div v-if="loadingList" class="space-y-2 p-4">
            <div class="h-16 animate-pulse rounded-lg bg-muted"></div>
            <div class="h-16 animate-pulse rounded-lg bg-muted"></div>
            <div class="h-16 animate-pulse rounded-lg bg-muted"></div>
          </div>
          <p v-else-if="conversations.length === 0" class="p-4 text-sm text-muted-foreground">
            {{ t('admin.support.listEmpty') }}
          </p>
          <button
            v-for="row in conversations"
            :key="row.id"
            type="button"
            class="w-full border-b border-border px-4 py-3 text-left transition-colors hover:bg-muted/60"
            :class="row.id === selectedId ? 'bg-accent' : ''"
            @click="selectConversation(row)"
          >
            <div class="flex items-start justify-between gap-2">
              <p class="truncate text-sm font-medium">{{ conversationTitle(row) }}</p>
              <span v-if="row.unread_for_agent > 0" class="theme-badge-danger shrink-0">
                {{ row.unread_for_agent }}
              </span>
            </div>
            <p class="mt-1 truncate text-xs text-muted-foreground">
              {{ row.last_message_preview || t('admin.support.previewEmpty') }}
            </p>
            <div class="mt-1 flex items-center justify-between gap-2 text-[11px] text-muted-foreground">
              <span>{{ formatDate(row.last_message_at || row.updated_at) }}</span>
              <span v-if="row.status === 'closed'" class="theme-badge-warning">
                {{ t('admin.support.filters.closed') }}
              </span>
            </div>
          </button>
        </div>

        <div class="border-t border-border p-3">
          <ListPagination
            :page="pagination.page"
            :total-page="pagination.total_page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            :page-size-options="pageSizeOptions"
            @change-page="changePage"
            @change-page-size="changePageSize"
          />
        </div>
      </aside>

      <section class="flex min-h-[520px] min-w-0 flex-1 flex-col rounded-xl border border-border bg-card shadow-sm">
        <div v-if="!selected" class="flex flex-1 items-center justify-center p-6 text-sm text-muted-foreground">
          {{ t('admin.support.selectHint') }}
        </div>

        <template v-else>
          <header class="flex items-start justify-between gap-3 border-b border-border p-4">
            <div class="min-w-0">
              <p class="truncate font-medium">{{ conversationTitle(selected) }}</p>
              <p class="mt-1 text-xs text-muted-foreground">
                <RouterLink
                  v-if="canOpenUser"
                  :to="`/users/${selected.user_id}`"
                  class="underline-offset-2 hover:underline"
                >
                  {{ t('admin.support.openUser', { id: selected.user_id }) }}
                </RouterLink>
                <span v-else>{{ t('admin.support.openUser', { id: selected.user_id }) }}</span>
              </p>
            </div>
            <Button
              v-if="canStatus"
              variant="outline"
              size="sm"
              :disabled="statusUpdating"
              @click="toggleStatus"
            >
              {{ isClosed ? t('admin.support.reopen') : t('admin.support.close') }}
            </Button>
          </header>

          <p v-if="isClosed" class="border-b border-border px-4 py-2 text-xs text-muted-foreground">
            {{ t('admin.support.closedHint') }}
          </p>

          <div ref="scrollRef" class="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-3">
            <div v-if="loadingMessages" class="flex h-full items-center justify-center">
              <Loader2 class="h-5 w-5 animate-spin text-muted-foreground" aria-hidden="true" />
            </div>
            <p v-else-if="messages.length === 0" class="py-12 text-center text-sm text-muted-foreground">
              {{ t('admin.support.messagesEmpty') }}
            </p>
            <div
              v-for="message in messages"
              v-else
              :key="message.id"
              class="flex"
              :class="message.sender_type === 'agent' ? 'justify-end' : 'justify-start'"
            >
              <div class="max-w-[78%] space-y-1">
                <div
                  class="rounded-2xl px-3 py-2 text-sm break-words"
                  :class="message.sender_type === 'agent'
                    ? 'rounded-br-sm bg-primary text-primary-foreground'
                    : 'rounded-bl-sm bg-muted text-foreground'"
                >
                  <template v-if="message.content_type === 'text'">
                    <p class="whitespace-pre-wrap">{{ message.content }}</p>
                    <template v-if="message.sender_type === 'user' && message.translation">
                      <div class="my-2 border-t border-border/70" />
                      <p class="whitespace-pre-wrap">{{ message.translation }}</p>
                    </template>
                  </template>
                  <a
                    v-else-if="message.content_type === 'image'"
                    :href="getImageUrl(message.media_url)"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="block"
                  >
                    <img
                      :src="getImageUrl(message.media_url)"
                      :alt="t('admin.support.imageAlt')"
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
                  :class="message.sender_type === 'agent' ? 'text-right' : 'text-left'"
                >
                  {{ message.sender_type === 'agent' ? t('admin.support.agent') : t('admin.support.buyer') }}
                  · {{ formatDate(message.created_at) }}
                </p>
              </div>
            </div>
          </div>

          <div class="border-t border-border p-3">
            <div class="mb-2">
              <div class="mb-1.5 flex items-center justify-between gap-2">
                <p class="text-xs text-muted-foreground">{{ t('admin.support.faqTitle') }}</p>
                <Button
                  v-if="canCreateQuickReply"
                  type="button"
                  variant="ghost"
                  size="sm"
                  class="h-7 px-2"
                  @click="openCreateQuickReply"
                >
                  <Plus class="mr-1 h-3.5 w-3.5" />
                  {{ t('admin.supportQuickReplies.create') }}
                </Button>
              </div>
              <p v-if="quickReplies.length === 0" class="text-xs text-muted-foreground">
                {{ t('admin.supportQuickReplies.empty') }}
              </p>
              <div v-else class="flex flex-wrap gap-1.5">
                <div
                  v-for="reply in (canWriteQuickReply ? quickReplies : activeQuickReplies)"
                  :key="reply.id"
                  class="inline-flex items-center gap-0.5 rounded-full border border-border"
                  :class="reply.is_active ? '' : 'opacity-60'"
                >
                  <button
                    type="button"
                    class="px-2.5 py-1 text-xs text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
                    :disabled="!canReply || composerBusy || !reply.is_active"
                    @click="fillQuickReply(reply)"
                  >
                    {{ reply.title }}
                  </button>
                  <button
                    v-if="canUpdateQuickReply"
                    type="button"
                    class="px-1.5 py-1 text-muted-foreground hover:text-foreground"
                    :aria-label="t('admin.common.edit')"
                    @click="openEditQuickReply(reply)"
                  >
                    <Pencil class="h-3 w-3" />
                  </button>
                  <button
                    v-if="canDeleteQuickReply"
                    type="button"
                    class="pr-2 py-1 text-muted-foreground hover:text-destructive"
                    :aria-label="t('admin.common.delete')"
                    @click="removeQuickReply(reply)"
                  >
                    <Trash2 class="h-3 w-3" />
                  </button>
                </div>
              </div>
            </div>
            <p v-if="formError" class="mb-2 text-xs text-destructive">{{ formError }}</p>
            <div class="flex items-end gap-2">
              <input
                ref="fileInput"
                type="file"
                class="hidden"
                accept="image/jpeg,image/png,image/gif,image/webp,video/mp4,video/webm,video/quicktime,.jpg,.jpeg,.png,.gif,.webp,.mp4,.webm,.mov"
                @change="handleFileChange"
              />
              <Button
                v-if="canReply && canUpload"
                type="button"
                variant="ghost"
                size="icon"
                :disabled="composerBusy"
                :aria-label="t('admin.support.attach')"
                @click="fileInput?.click()"
              >
                <Loader2 v-if="uploading" class="h-4 w-4 animate-spin" />
                <Paperclip v-else class="h-4 w-4" />
              </Button>
              <Textarea
                v-model="draft"
                class="min-h-[72px]"
                :disabled="!canReply || composerBusy"
                :placeholder="t('admin.support.composerPlaceholder')"
                @keydown="handleKeydown"
              />
              <Button
                type="button"
                :disabled="!canReply || composerBusy || !draft.trim()"
                @click="sendText"
              >
                <Loader2 v-if="sending" class="mr-1 h-4 w-4 animate-spin" />
                <SendHorizontal v-else class="mr-1 h-4 w-4" />
                {{ t('admin.support.send') }}
              </Button>
            </div>
          </div>
        </template>
      </section>

      <!-- 订单上下文侧栏：F-44 要求点开会话即可见该买家的订单 -->
      <aside
        v-if="canViewOrders && selected"
        class="flex w-full flex-col rounded-xl border border-border bg-card shadow-sm lg:max-w-xs"
      >
        <div class="border-b border-border p-4">
          <p class="text-sm font-semibold">{{ t('admin.support.orders.title') }}</p>
          <p class="mt-1 text-xs text-muted-foreground">
            {{ t('admin.support.orders.subtitle', { id: selected.user_id }) }}
          </p>
        </div>

        <div class="min-h-0 flex-1 overflow-y-auto p-3">
          <div v-if="loadingOrders" class="flex h-24 items-center justify-center">
            <Loader2 class="h-4 w-4 animate-spin text-muted-foreground" aria-hidden="true" />
          </div>
          <p v-else-if="buyerOrders.length === 0" class="px-1 py-6 text-center text-xs text-muted-foreground">
            {{ t('admin.support.orders.empty') }}
          </p>
          <ul v-else class="space-y-2">
            <li
              v-for="order in buyerOrders"
              :key="order.id"
              class="rounded-lg border border-border p-3"
            >
              <div class="flex items-start justify-between gap-2">
                <RouterLink
                  v-if="canOpenOrder"
                  :to="`/orders?order_id=${order.id}`"
                  class="min-w-0 font-mono text-xs underline-offset-2 hover:underline"
                >
                  {{ order.order_no }}
                </RouterLink>
                <span v-else class="min-w-0 break-all font-mono text-xs">{{ order.order_no }}</span>
                <span
                  class="shrink-0 rounded-full border px-2 py-0.5 text-[10px]"
                  :class="orderStatusClass(order.status)"
                >
                  {{ orderStatusText(order.status) }}
                </span>
              </div>
              <div class="mt-2 flex items-center justify-between gap-2 text-xs text-muted-foreground">
                <span class="font-mono">{{ formatMoney(order.total_amount, order.currency) }}</span>
                <span>{{ formatDate(order.created_at) }}</span>
              </div>
            </li>
          </ul>
        </div>
      </aside>
    </div>

    <Dialog v-model:open="showQuickReplyModal" @update:open="(value: boolean) => { if (!value) closeQuickReplyModal() }">
      <DialogScrollContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {{ editingQuickReplyId ? t('admin.supportQuickReplies.edit') : t('admin.supportQuickReplies.create') }}
          </DialogTitle>
        </DialogHeader>
        <div class="space-y-4">
          <p v-if="quickReplyFormError" class="text-xs text-destructive">{{ quickReplyFormError }}</p>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.supportQuickReplies.form.title') }}
            </label>
            <Input v-model="quickReplyForm.title" maxlength="120" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.supportQuickReplies.form.titleHint') }}</p>
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.supportQuickReplies.form.content') }}
            </label>
            <Textarea v-model="quickReplyForm.content" class="min-h-[120px]" />
            <p class="mt-1 text-xs text-muted-foreground">{{ t('admin.supportQuickReplies.form.contentHint') }}</p>
          </div>
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm font-medium">{{ t('admin.supportQuickReplies.form.isActive') }}</p>
            <Switch v-model="quickReplyForm.is_active" />
          </div>
          <div>
            <label class="mb-1.5 block text-xs font-medium text-muted-foreground">
              {{ t('admin.supportQuickReplies.form.sortOrder') }}
            </label>
            <Input
              type="number"
              step="1"
              class="sm:max-w-xs"
              :model-value="quickReplyForm.sort_order"
              @update:model-value="quickReplyForm.sort_order = asInt($event)"
            />
          </div>
          <div class="flex justify-end gap-2">
            <Button variant="outline" @click="closeQuickReplyModal">{{ t('admin.common.cancel') }}</Button>
            <Button :disabled="quickReplySubmitting" @click="submitQuickReply">{{ t('admin.common.save') }}</Button>
          </div>
        </div>
      </DialogScrollContent>
    </Dialog>
  </div>
</template>
