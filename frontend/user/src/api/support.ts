import { userApi } from './client'
import type { SupportSendMessagePayload } from './types'

export const supportAPI = {
    conversation: () => userApi.get('/support/conversation'),
    // after_id 为增量游标：只返回该 id 之后的新消息，用于轮询。
    messages: (params?: { after_id?: number; page?: number; page_size?: number }) =>
        userApi.get('/support/messages', { params }),
    send: (data: SupportSendMessagePayload) => userApi.post('/support/messages', data),
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
