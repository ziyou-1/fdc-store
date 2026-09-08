import { describe, it, expect } from 'vitest'
import {
  permissionDescription,
  permissionDescriptions,
  permissionDescriptionCount,
} from './authzPermissionDescriptions'

describe('permissionDescriptions', () => {
  it('覆盖全部 276 个后台接口权限', () => {
    expect(Object.keys(permissionDescriptions).length).toBe(permissionDescriptionCount)
    expect(permissionDescriptionCount).toBe(276)
  })

  it('三种语言都有非空说明', () => {
    for (const [key, entry] of Object.entries(permissionDescriptions)) {
      for (const locale of ['zh-CN', 'zh-TW', 'en-US'] as const) {
        expect(entry[locale]?.trim().length, `${key} 的 ${locale} 为空`).toBeGreaterThan(0)
      }
    }
  })

  // 说明要讲清「对什么做什么」。只回显模块名（例如「商品管理」）等于没说，
  // 这里用长度做个粗筛，挡住明显敷衍的条目。
  it('中文说明不是干巴巴的名词', () => {
    for (const [key, entry] of Object.entries(permissionDescriptions)) {
      expect(entry['zh-CN'].length, `${key} 的中文说明过短`).toBeGreaterThanOrEqual(4)
    }
  })

  it('危险操作要点明后果', () => {
    expect(permissionDescription('DELETE:/admin/products/:id', 'zh-CN')).toContain('删除')
    expect(permissionDescription('POST:/admin/channel-clients/:id/reset-secret', 'zh-CN')).toContain('失效')
    expect(permissionDescription('DELETE:/admin/payment-channels/:id', 'zh-CN')).toContain('结算')
  })

  it('按语言取值，未知语言回落简体', () => {
    const key = 'GET:/admin/products'
    expect(permissionDescription(key, 'en-US')).toBe(permissionDescriptions[key]!['en-US'])
    expect(permissionDescription(key, 'xx-YY')).toBe(permissionDescriptions[key]!['zh-CN'])
  })

  // 后端新增接口时这里会缺项，必须返回空串让调用方回落，而不是抛错或返回 undefined
  it('未收录的权限返回空串', () => {
    expect(permissionDescription('GET:/admin/brand-new-endpoint', 'zh-CN')).toBe('')
    expect(permissionDescription('', 'zh-CN')).toBe('')
  })
})
