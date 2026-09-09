import { userApi } from './client'
import type { SupportSendMessagePayload } from './types'

export const supportAPI = {
    conversation: () => userApi.get('/support/conversation'),
    // 两条独立游标：after_id 拉新消息，revoked_after 拉撤回。
    // 撤回不产生新消息，只靠 after_id 永远拉不到，不刷新就看不到对方撤回。
    messages: (params?: {
        after_id?: number
        page?: number
        page_size?: number
        revoked_after?: string
    }) => userApi.get('/support/messages', { params }),
    send: (data: SupportSendMessagePayload) => userApi.post('/support/messages', data),
    // 只能撤自己 2 分钟内发的，服务端判定；越权或超时返回可读错误。
    revoke: (id: number) => userApi.post(`/support/messages/${id}/revoke`),
    markRead: () => userApi.post('/support/messages/read'),
    quickReplies: () => userApi.get('/support/quick-replies'),
    // 字段名必须是 file，后端 c.FormFile("file") 硬编码。
    // 附件上限 50MB（视频），默认 10s 超时不够，显式放宽到 2 分钟。
    uploadAttachment: (file: File) => {
        const formData = new FormData()
        formData.append('file', file)
        return userApi.post('/support/attachments', formData, { timeout: 120000 })
    },
}
