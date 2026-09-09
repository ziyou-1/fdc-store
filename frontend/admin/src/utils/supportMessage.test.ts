import { describe, expect, it } from 'vitest'
import type { AdminSupportMessage } from '@/api/types'
import {
  REVOKE_WINDOW_MS,
  canRevokeMessage,
  isRevoked,
  mergeRevocations,
  nextRevokedCursor,
  quoteSummary,
} from './supportMessage'

const LABELS = { revoked: '已撤回', image: '[图片]', video: '[视频]' }

const NOW = new Date('2026-09-09T10:00:00Z').getTime()

const agentMessage = (overrides: Record<string, unknown> = {}) => ({
  sender_type: 'agent',
  content_type: 'text',
  content: '你好',
  revoked_at: null,
  created_at: new Date(NOW).toISOString(),
  ...overrides,
})

describe('撤回按钮的可见性', () => {
  it('客服刚发出的消息可以撤回', () => {
    expect(canRevokeMessage(agentMessage() as never, NOW + 1000)).toBe(true)
  })

  it('窗口边界（正好 2 分钟）仍可撤回', () => {
    expect(canRevokeMessage(agentMessage() as never, NOW + REVOKE_WINDOW_MS)).toBe(true)
  })

  it('超过 2 分钟就不再显示撤回', () => {
    expect(canRevokeMessage(agentMessage() as never, NOW + REVOKE_WINDOW_MS + 1)).toBe(false)
  })

  it('买家消息永远不能撤回——客服撤掉买家说过的话等于篡改证据', () => {
    const buyer = agentMessage({ sender_type: 'user' })
    expect(canRevokeMessage(buyer as never, NOW + 1000)).toBe(false)
  })

  it('已撤回的消息不再显示撤回按钮', () => {
    const revoked = agentMessage({ revoked_at: new Date(NOW).toISOString() })
    expect(canRevokeMessage(revoked as never, NOW + 1000)).toBe(false)
  })

  it('created_at 解析不出来时按不可撤回处理，而不是把按钮永久点亮', () => {
    const broken = agentMessage({ created_at: 'not-a-date' })
    expect(canRevokeMessage(broken as never, NOW)).toBe(false)
  })
})

describe('引用块摘要', () => {
  it('被引用的消息已撤回时，摘要不能泄露原文', () => {
    const revoked = agentMessage({ content: '这句话已经撤回了', revoked_at: new Date(NOW).toISOString() })
    const summary = quoteSummary(revoked as never, LABELS)

    expect(summary).toBe('已撤回')
    expect(summary).not.toContain('这句话已经撤回了')
  })

  it('图片与视频没有文字，用占位标签代替', () => {
    expect(quoteSummary(agentMessage({ content_type: 'image', content: '' }) as never, LABELS)).toBe('[图片]')
    expect(quoteSummary(agentMessage({ content_type: 'video', content: '' }) as never, LABELS)).toBe('[视频]')
  })

  it('普通文本原样返回', () => {
    expect(quoteSummary(agentMessage({ content: '订单没到' }) as never, LABELS)).toBe('订单没到')
  })
})

describe('撤回态判定', () => {
  it('revoked_at 为空串或 null 都算未撤回', () => {
    expect(isRevoked({ revoked_at: null } as never)).toBe(false)
    expect(isRevoked({ revoked_at: '' } as never)).toBe(false)
    expect(isRevoked({ revoked_at: undefined } as never)).toBe(false)
  })

  it('revoked_at 有值即为已撤回', () => {
    expect(isRevoked({ revoked_at: '2026-09-09T10:00:00Z' } as never)).toBe(true)
  })
})

// ---- 撤回增量的合并：不刷新也能看到对方撤回 ----

const listMessage = (overrides: Partial<AdminSupportMessage> = {}): AdminSupportMessage => ({
  id: 1,
  conversation_id: 1,
  sender_type: 'user',
  sender_id: 7,
  content_type: 'text',
  content: '你好',
  translation: '你好',
  media_url: '',
  revoked_at: null,
  created_at: new Date(NOW).toISOString(),
  ...overrides,
})

describe('撤回增量的合并', () => {
  it('就地把已有消息标成已撤回，并清掉本地残留的原文与译文', () => {
    const list = [listMessage({ id: 1 }), listMessage({ id: 2, content: '要撤的' })]

    const next = mergeRevocations(list, [{ id: 2, revoked_at: '2026-09-09T10:01:00Z' }])

    expect(next[1]?.revoked_at).toBe('2026-09-09T10:01:00Z')
    expect(next[1]?.content).toBe('')
    expect(next[1]?.translation).toBe('')
    expect(next[0]?.revoked_at).toBeNull()
  })

  // 撤回的消息可能已经滑出本地加载的那一页。按「没见过就追加」处理的话，
  // 列表里会凭空多出一条空气泡，还会因为数组变长触发滚动到底。
  it('绝不新增本地没有的消息', () => {
    const list = [listMessage({ id: 1 })]

    const next = mergeRevocations(list, [{ id: 999, revoked_at: '2026-09-09T10:01:00Z' }])

    expect(next).toHaveLength(1)
    expect(next[0]?.id).toBe(1)
  })

  it('没有任何变化时返回原数组，避免整列表重渲', () => {
    const list = [listMessage({ id: 1 })]

    expect(mergeRevocations(list, [])).toBe(list)
    expect(mergeRevocations(list, undefined)).toBe(list)
    expect(mergeRevocations(list, [{ id: 42, revoked_at: 'x' }])).toBe(list)
  })

  it('已经是撤回态的不重复覆盖', () => {
    const list = [listMessage({ id: 1, revoked_at: '2026-09-09T10:00:00Z' })]

    expect(mergeRevocations(list, [{ id: 1, revoked_at: '2026-09-09T10:05:00Z' }])).toBe(list)
  })
})

describe('撤回游标', () => {
  // 游标退回空值会让下一轮把全部历史撤回再拉一遍。
  it('服务端本轮没给就保留旧值', () => {
    expect(nextRevokedCursor('2026-09-09T10:00:00Z', null)).toBe('2026-09-09T10:00:00Z')
    expect(nextRevokedCursor('2026-09-09T10:00:00Z', undefined)).toBe('2026-09-09T10:00:00Z')
    expect(nextRevokedCursor('2026-09-09T10:00:00Z', '')).toBe('2026-09-09T10:00:00Z')
  })

  it('给了新游标就往前推', () => {
    expect(nextRevokedCursor('', '2026-09-09T10:00:00Z')).toBe('2026-09-09T10:00:00Z')
    expect(nextRevokedCursor('2026-09-09T10:00:00Z', '2026-09-09T10:05:00Z')).toBe('2026-09-09T10:05:00Z')
  })
})
