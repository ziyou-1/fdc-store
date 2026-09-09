import type { SupportMessageData } from '../api/types'

/**
 * 撤回窗口，与后端 supportdomain.RevokeWindow 保持一致（2 分钟）。
 *
 * 前端只负责决定按钮显不显示，真正的判定在服务端——
 * 改这个常量不会放宽任何权限，只会让按钮点下去被服务端拒绝。
 */
export const REVOKE_WINDOW_MS = 2 * 60 * 1000

export const isRevoked = (message: Pick<SupportMessageData, 'revoked_at'>): boolean =>
  Boolean(message.revoked_at)

/**
 * canRevokeMessage 判断撤回按钮该不该出现。
 *
 * mine 必须显式传，不能只看「是不是我这一侧」的布尔值：买家 ID 与管理员 ID
 * 是两套独立自增序列、共用 sender_id 一列，后端也是靠 sender_type + sender_id
 * 两个一起判的（domain.CanRevokeBy）。前端跟它对齐，避免两边规则悄悄分叉。
 */
export const canRevokeMessage = (
  message: Pick<SupportMessageData, 'sender_type' | 'revoked_at' | 'created_at'>,
  now: number,
  mine: 'user' | 'agent',
): boolean => {
  if (message.sender_type !== mine || isRevoked(message)) return false
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
  message: Pick<SupportMessageData, 'content_type' | 'content' | 'revoked_at'>,
  labels: { revoked: string; image: string; video: string },
): string => {
  if (isRevoked(message)) return labels.revoked
  if (message.content_type === 'image') return labels.image
  if (message.content_type === 'video') return labels.video
  return message.content
}

/**
 * mergeRevocations 把服务端回传的撤回增量打到已有列表上。
 *
 * **只更新已有的，绝不新增。** 撤回窗口是 2 分钟，被撤的消息通常就在列表尾部；
 * 但快节奏会话里它可能已经滑出客户端持有的那一页——那时若按「没见过就追加」
 * 处理，列表里会凭空多出一条内容为空的气泡，还会因为 length 变化触发滚动到底。
 * 没加载到的那条，下次整页拉取时本来就带着 revoked_at 回来。
 */
export const mergeRevocations = <T extends SupportMessageData>(
  messages: T[],
  revoked: Array<Pick<SupportMessageData, 'id' | 'revoked_at'>>,
): T[] => {
  if (!revoked || revoked.length === 0) return messages
  const byID = new Map(revoked.map((item) => [item.id, item]))
  let changed = false
  const next = messages.map((message) => {
    const hit = byID.get(message.id)
    if (!hit || message.revoked_at) return message
    changed = true
    // 内容服务端已经抹过，这里只补上撤回时间戳并清掉本地残留的原文。
    return { ...message, revoked_at: hit.revoked_at, content: '', media_url: '' }
  })
  // 没有任何变化就返回原数组：换新引用会让 Vue 白白重渲整个列表。
  return changed ? next : messages
}

/**
 * nextRevokedCursor 决定下一轮要带回服务端的撤回游标。
 *
 * 游标由服务端产出、前端原样回显，所以不受两边时钟不一致影响。
 * 服务端本轮没给（没有任何撤回）就保留旧值——退回空值会让下一轮把
 * 全部历史撤回再拉一遍。
 */
export const nextRevokedCursor = (
  current: string,
  incoming?: string | null,
): string => incoming || current
