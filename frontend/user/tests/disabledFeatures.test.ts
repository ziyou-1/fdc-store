import test from 'node:test'
import assert from 'node:assert/strict'
import { disabledFeatures, isDisabledFeaturePath, isDisabledPersonalSection } from '../src/utils/disabledFeatures.ts'

// 这里只断言「不随运营开关变化」的不变量。
// 各功能的开关值本身经常调整，逐个硬编码会让这个测试变成噪音。

test('后端未就绪的功能必须在前端一并挡住', () => {
  assert.equal(isDisabledFeaturePath('/reseller/orders'), true)
  assert.equal(isDisabledFeaturePath('/me/reseller'), true)
  assert.equal(isDisabledFeaturePath('/auth/telegram'), true)
  assert.equal(isDisabledFeaturePath('/auth/google'), true)
})

test('核心购物路径永远不得被开关误挡', () => {
  for (const path of ['/', '/products', '/cart', '/checkout', '/me', '/me/orders', '/me/membership']) {
    assert.equal(isDisabledFeaturePath(path), false, `${path} 不该被拦截`)
  }
})

test('游客功能已彻底删除：不留开关，也不需要路径拦截', () => {
  assert.equal('guestOrders' in disabledFeatures, false)
  // 路由本身已不存在，命中 404 即可，不该再靠前缀表兜底
  assert.equal(isDisabledFeaturePath('/guest/orders'), false)
})

test('博客已彻底删除：不留开关', () => {
  assert.equal('blog' in disabledFeatures, false)
})

test('停用的个人中心分区与开关保持一致', () => {
  assert.equal(isDisabledPersonalSection('reseller'), true)
  assert.equal(isDisabledPersonalSection('orders'), false)
  assert.equal(isDisabledPersonalSection('profile'), false)
})
