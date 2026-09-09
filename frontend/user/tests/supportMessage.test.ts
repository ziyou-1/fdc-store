import test from 'node:test'
import assert from 'node:assert/strict'
import {
  REVOKE_WINDOW_MS,
  canRevokeMessage,
  isRevoked,
  mergeRevocations,
  nextRevokedCursor,
  quoteSummary,
} from '../src/utils/supportMessage.ts'

const NOW = new Date('2026-09-09T10:00:00Z').getTime()

const message = (overrides: Record<string, unknown> = {}) =>
  ({
    id: 1,
    conversation_id: 1,
    sender_type: 'user',
    sender_id: 7,
    content_type: 'text',
    content: '你好',
    media_url: '',
    revoked_at: null,
    created_at: new Date(NOW).toISOString(),
    ...overrides,
  }) as never

test('买家能撤自己刚发的消息', () => {
  assert.equal(canRevokeMessage(message(), NOW + 1000, 'user'), true)
})

test('窗口边界（正好 2 分钟）仍可撤回', () => {
  assert.equal(canRevokeMessage(message(), NOW + REVOKE_WINDOW_MS, 'user'), true)
  assert.equal(canRevokeMessage(message(), NOW + REVOKE_WINDOW_MS + 1, 'user'), false)
})

// 买家撤不了客服的消息，也不该出现按钮。
test('客服消息在买家端不显示撤回', () => {
  assert.equal(canRevokeMessage(message({ sender_type: 'agent' }), NOW + 1000, 'user'), false)
})

test('已撤回的不再显示撤回按钮', () => {
  const revoked = message({ revoked_at: new Date(NOW).toISOString() })
  assert.equal(canRevokeMessage(revoked, NOW + 1000, 'user'), false)
})

test('created_at 解析不出来时按不可撤回处理，而不是把按钮永久点亮', () => {
  assert.equal(canRevokeMessage(message({ created_at: 'not-a-date' }), NOW, 'user'), false)
})

// 撤回后原文不能从引用块里漏出来。
test('引用已撤回的消息，摘要不泄露原文', () => {
  const revoked = message({ content: '这句话撤回了', revoked_at: new Date(NOW).toISOString() })
  const summary = quoteSummary(revoked, { revoked: '已撤回', image: '[图片]', video: '[视频]' })

  assert.equal(summary, '已撤回')
  assert.ok(!summary.includes('这句话撤回了'))
})

test('图片与视频用占位标签代替', () => {
  const labels = { revoked: '已撤回', image: '[图片]', video: '[视频]' }
  assert.equal(quoteSummary(message({ content_type: 'image', content: '' }), labels), '[图片]')
  assert.equal(quoteSummary(message({ content_type: 'video', content: '' }), labels), '[视频]')
})

test('revoked_at 有值即为已撤回', () => {
  assert.equal(isRevoked({ revoked_at: null } as never), false)
  assert.equal(isRevoked({ revoked_at: '' } as never), false)
  assert.equal(isRevoked({ revoked_at: '2026-09-09T10:00:00Z' } as never), true)
})

// ---- 撤回增量的合并：用户报的「撤回后要刷新一下才看得到」就是这里 ----

test('撤回增量能就地把已有消息标成已撤回', () => {
  const list = [message({ id: 1 }), message({ id: 2, content: '要撤的' })]

  const next = mergeRevocations(list as never[], [{ id: 2, revoked_at: '2026-09-09T10:01:00Z' }])

  assert.equal(next[1].revoked_at, '2026-09-09T10:01:00Z')
  // 本地残留的原文也要清掉，否则引用块和气泡还能读到
  assert.equal(next[1].content, '')
  assert.equal(next[0].revoked_at, null)
})

// 这条是关键：撤回的消息可能已经滑出本地加载的那一页。
// 若按「没见过就追加」处理，列表里会凭空多出一条空气泡。
test('撤回增量绝不新增本地没有的消息', () => {
  const list = [message({ id: 1 })]

  const next = mergeRevocations(list as never[], [{ id: 999, revoked_at: '2026-09-09T10:01:00Z' }])

  assert.equal(next.length, 1)
  assert.equal(next[0].id, 1)
})

test('没有任何变化时返回原数组，避免整列表重渲', () => {
  const list = [message({ id: 1 })] as never[]

  assert.equal(mergeRevocations(list, []), list)
  assert.equal(mergeRevocations(list, undefined as never), list)
  // 目标不在本地：同样不该换引用
  assert.equal(mergeRevocations(list, [{ id: 42, revoked_at: 'x' }]), list)
})

test('已经是撤回态的不重复覆盖', () => {
  const list = [message({ id: 1, revoked_at: '2026-09-09T10:00:00Z' })] as never[]

  assert.equal(mergeRevocations(list, [{ id: 1, revoked_at: '2026-09-09T10:05:00Z' }]), list)
})

// 游标退回空值会让下一轮把全部历史撤回再拉一遍。
test('服务端本轮没给游标时保留旧值', () => {
  assert.equal(nextRevokedCursor('2026-09-09T10:00:00Z', null), '2026-09-09T10:00:00Z')
  assert.equal(nextRevokedCursor('2026-09-09T10:00:00Z', undefined), '2026-09-09T10:00:00Z')
  assert.equal(nextRevokedCursor('2026-09-09T10:00:00Z', ''), '2026-09-09T10:00:00Z')
})

test('服务端给了新游标就往前推', () => {
  assert.equal(nextRevokedCursor('', '2026-09-09T10:00:00Z'), '2026-09-09T10:00:00Z')
  assert.equal(
    nextRevokedCursor('2026-09-09T10:00:00Z', '2026-09-09T10:05:00Z'),
    '2026-09-09T10:05:00Z',
  )
})
