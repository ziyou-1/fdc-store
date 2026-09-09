import type { AdminSupportMessage } from '@/api/types'

/**
 * 撤回窗口，与后端 supportdomain.RevokeWindow 保持一致（2 分钟）。
 *
 * 前端只负责决定按钮显不显示，真正的判定在服务端——
 * 改这个常量不会放宽任何权限，只会让按钮点下去被服务端拒绝。
 */
export const REVOKE_WINDOW_MS = 2 * 60 * 1000

export const isRevoked = (message: Pick<AdminSupportMessage, 'revoked_at'>): boolean =>
  Boolean(message.revoked_at)

/**
 * canRevokeMessage 判断撤回按钮该不该出现。
 *
 * 只看「客服发的 + 未撤回 + 在窗口内」三条。
 * 「是不是本人发的」故意不在这里判：后台前端拿不到当前管理员 ID
 * （authStore 只存 token 与权限串），越权时由服务端返回可读错误。
 */
export const canRevokeMessage = (
  message: Pick<AdminSupportMessage, 'sender_type' | 'revoked_at' | 'created_at'>,
  now: number,
): boolean => {
  if (message.sender_type !== 'agent' || isRevoked(message)) return false
  const sentAt = new Date(message.created_at).getTime()
  if (!Number.isFinite(sentAt)) return false
  return now - sentAt <= REVOKE_WINDOW_MS
}

/**
 * quoteSummary 生成引用块里的那一行摘要。
 *
 * 撤回优先于一切：被引用的消息一旦撤回，摘要必须换成「已撤回」文案，
 * 否则原文会从引用块里漏出来——那就等于撤回没生效。
 */
export const quoteSummary = (
  message: Pick<AdminSupportMessage, 'content_type' | 'content' | 'revoked_at'>,
  labels: { revoked: string; image: string; video: string },
): string => {
  if (isRevoked(message)) return labels.revoked
  if (message.content_type === 'image') return labels.image
  if (message.content_type === 'video') return labels.video
  return message.content
}
