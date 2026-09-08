import { describe, it, expect } from 'vitest'
import { summarizeRolePolicies, capabilityActionText, moduleKeyOf } from './authzSummary'

describe('moduleKeyOf', () => {
  it('与后端同规则地从路径推出模块键', () => {
    expect(moduleKeyOf('/admin/products/:id')).toBe('products')
    expect(moduleKeyOf('/admin/authz/policies')).toBe('authz')
    expect(moduleKeyOf('/admin/authz/roles/:role/policies')).toBe('authz')
    expect(moduleKeyOf('')).toBe('system')
  })
})

describe('summarizeRolePolicies', () => {
  it('按模块归并，并把动作合并成集合', () => {
    const caps = summarizeRolePolicies(
      [
        { object: '/admin/products', action: 'GET' },
        { object: '/admin/products/:id', action: 'DELETE' },
        { object: '/admin/products/:id', action: 'PUT' },
        { object: '/admin/posts', action: 'GET' },
      ],
      'zh-CN',
    )
    const products = caps.find((c) => c.module === 'products')!
    expect(products.count).toBe(3)
    expect(products.actions).toEqual(['GET', 'PUT', 'DELETE'])
    expect(products.label).toBe('商品管理')
    expect(caps.find((c) => c.module === 'posts')!.count).toBe(1)
  })

  // 有通配就不该再罗列具体动作，否则「全部操作、查看、删除」读起来自相矛盾
  it('通配权限吞掉具体动作', () => {
    const caps = summarizeRolePolicies(
      [
        { object: '/admin/products', action: '*' },
        { object: '/admin/products/:id', action: 'GET' },
      ],
      'zh-CN',
    )
    const products = caps.find((c) => c.module === 'products')!
    expect(products.full).toBe(true)
    expect(products.actions).toEqual(['*'])
    expect(capabilityActionText(products, 'zh-CN')).toBe('全部操作')
  })

  // 涉及钱、密钥、权限的模块要排在前面并标危险，否则埋在三十行里没人注意
  it('危险模块排前面并被标记', () => {
    const caps = summarizeRolePolicies(
      [
        { object: '/admin/posts', action: 'GET' },
        { object: '/admin/posts/:id', action: 'GET' },
        { object: '/admin/posts/:id/x', action: 'GET' },
        { object: '/admin/payment-channels/:id', action: 'DELETE' },
      ],
      'zh-CN',
    )
    expect(caps[0]!.module).toBe('payment-channels')
    expect(caps[0]!.dangerous).toBe(true)
    expect(caps.find((c) => c.module === 'posts')!.dangerous).toBe(false)
  })

  it('只读权限不算危险', () => {
    const caps = summarizeRolePolicies([{ object: '/admin/payments', action: 'GET' }], 'zh-CN')
    expect(caps[0]!.dangerous).toBe(false)
  })

  // PUT 与 PATCH 都译作「修改」，不能渲染成「修改、修改」
  it('同义动作去重', () => {
    const caps = summarizeRolePolicies(
      [
        { object: '/admin/products/:id', action: 'PUT' },
        { object: '/admin/products/:id/x', action: 'PATCH' },
      ],
      'zh-CN',
    )
    expect(capabilityActionText(caps[0]!, 'zh-CN')).toBe('修改')
  })

  it('空策略返回空摘要', () => {
    expect(summarizeRolePolicies([], 'zh-CN')).toEqual([])
  })
})
