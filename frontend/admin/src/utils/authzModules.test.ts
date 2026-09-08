import { describe, it, expect } from 'vitest'
import { authzModuleLabel, authzMethodHint, knownAuthzModuleKeys } from './authzModules'

describe('authzModuleLabel', () => {
  it('把后端推导出的模块键翻成人话', () => {
    expect(authzModuleLabel('api-credentials', 'zh-CN')).toBe('开放接口凭证')
    expect(authzModuleLabel('payment-channels', 'zh-CN')).toBe('支付渠道')
    expect(authzModuleLabel('api-credentials', 'zh-TW')).toBe('開放介面憑證')
    expect(authzModuleLabel('api-credentials', 'en-US')).toBe('API Credentials')
  })

  // 后端新增接口时会冒出没收录的模块键。宁可显示英文原键，
  // 也不能显示空白——空白会让运营以为权限丢了。
  it('未收录的键原样返回，不返回空白', () => {
    expect(authzModuleLabel('brand-new-module', 'zh-CN')).toBe('brand-new-module')
    expect(authzModuleLabel('', 'zh-CN')).toBe('')
  })

  it('未知语言回落简体中文', () => {
    expect(authzModuleLabel('orders', 'xx-YY')).toBe('订单管理')
  })

  // 三种后台语言必须覆盖同一批键，漏配会让某个语言下露出英文原键。
  it('三种语言的模块键完全一致', () => {
    for (const key of knownAuthzModuleKeys) {
      for (const locale of ['zh-CN', 'zh-TW', 'en-US']) {
        const label = authzModuleLabel(key, locale)
        expect(label, `${locale} 缺少 ${key} 的标签`).not.toBe(key)
        expect(label.trim().length, `${locale} 的 ${key} 标签为空`).toBeGreaterThan(0)
      }
    }
  })
})

describe('authzMethodHint', () => {
  it('给 HTTP 方法配中文说明', () => {
    expect(authzMethodHint('GET', 'zh-CN')).toBe('查看')
    expect(authzMethodHint('delete', 'zh-CN')).toBe('删除')
    expect(authzMethodHint('*', 'zh-CN')).toBe('全部')
    expect(authzMethodHint('GET', 'en-US')).toBe('Read')
  })

  it('未知方法返回空串，由调用方回落原值', () => {
    expect(authzMethodHint('TRACE', 'zh-CN')).toBe('')
  })
})
