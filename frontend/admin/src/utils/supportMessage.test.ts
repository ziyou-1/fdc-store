import { describe, expect, it } from 'vitest'
import { REVOKE_WINDOW_MS, canRevokeMessage, isRevoked, quoteSummary } from './supportMessage'

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
