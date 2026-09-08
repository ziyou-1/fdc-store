// 角色策略在页面上是一行行原始路径，配了三十几条之后没人读得懂
// 「这个角色到底能干什么」。这里把策略按模块归并成一句话摘要，
// 让运营扫一眼就知道权限范围，同时把危险权限单独挑出来提醒。

import { authzModuleLabel } from './authzModules'

export interface AuthzPolicyLike {
  object: string
  action: string
}

/** 与后端 deriveAdminPermissionModule 同规则：/admin/<模块>/... 取第二段。 */
export const moduleKeyOf = (object: string): string => {
  const segments = String(object || '')
    .replace(/^\//, '')
    .split('/')
    .filter(Boolean)
  if (segments.length === 0) return 'system'
  if (segments.length === 1 || segments[0] !== 'admin') return segments[0] ?? 'system'
  return segments[1] === 'authz' ? 'authz' : (segments[1] ?? 'system')
}

/** 一个模块下这个角色拿到了哪几类动作。 */
export interface ModuleCapability {
  module: string
  label: string
  /** 是否持有该模块任意路径的通配权限 */
  full: boolean
  /** 归并后的动作集合，顺序固定便于阅读 */
  actions: string[]
  /** 该模块下的策略条数 */
  count: number
  /** 命中危险动作（删除、打款、改密钥、改权限） */
  dangerous: boolean
}

const ACTION_ORDER = ['*', 'GET', 'POST', 'PUT', 'PATCH', 'DELETE']

const ACTION_TEXT: Record<string, Record<string, string>> = {
  'zh-CN': { '*': '全部操作', GET: '查看', POST: '新增/操作', PUT: '修改', PATCH: '修改', DELETE: '删除' },
  'zh-TW': { '*': '全部操作', GET: '檢視', POST: '新增/操作', PUT: '修改', PATCH: '修改', DELETE: '刪除' },
  'en-US': { '*': 'Full access', GET: 'Read', POST: 'Create/Act', PUT: 'Update', PATCH: 'Update', DELETE: 'Delete' },
}

// 拿到这些就能造成不可逆后果或直接影响钱，摘要里单独标出来。
const DANGEROUS_MODULES = new Set([
  'authz',
  'payment-channels',
  'payments',
  'order-refunds',
  'wallet',
  'affiliates',
  'api-credentials',
  'channel-clients',
  'users',
  'settings',
])

export const summarizeRolePolicies = (
  policies: AuthzPolicyLike[],
  locale: string,
): ModuleCapability[] => {
  const grouped = new Map<string, { actions: Set<string>; count: number }>()
  for (const policy of policies || []) {
    const key = moduleKeyOf(policy.object)
    let entry = grouped.get(key)
    if (!entry) {
      entry = { actions: new Set<string>(), count: 0 }
      grouped.set(key, entry)
    }
    entry.actions.add(String(policy.action || '').toUpperCase() === '*' ? '*' : String(policy.action || '').toUpperCase())
    entry.count += 1
  }

  const out: ModuleCapability[] = []
  for (const [module, entry] of grouped) {
    const full = entry.actions.has('*')
    // 有通配就不必再罗列具体动作，否则「全部操作、查看、删除」读起来自相矛盾
    const actions = full
      ? ['*']
      : ACTION_ORDER.filter((a) => a !== '*' && entry.actions.has(a))
    out.push({
      module,
      label: authzModuleLabel(module, locale),
      full,
      actions,
      count: entry.count,
      dangerous: DANGEROUS_MODULES.has(module) && (full || entry.actions.has('DELETE') || entry.actions.has('POST') || entry.actions.has('PUT') || entry.actions.has('PATCH')),
    })
  }
  // 危险的排前面，其次按策略条数，最后按名称，保证同一角色每次渲染顺序一致
  out.sort((a, b) => {
    if (a.dangerous !== b.dangerous) return a.dangerous ? -1 : 1
    if (a.count !== b.count) return b.count - a.count
    return a.label.localeCompare(b.label)
  })
  return out
}

/** 把一个模块的动作集合渲染成「查看、修改、删除」这样的短语。 */
export const capabilityActionText = (cap: ModuleCapability, locale: string): string => {
  const map = ACTION_TEXT[locale] || ACTION_TEXT['zh-CN']
  const seen = new Set<string>()
  const words: string[] = []
  for (const action of cap.actions) {
    const word = (map && map[action]) || action
    // PUT 与 PATCH 都译作「修改」，去重免得出现「修改、修改」
    if (seen.has(word)) continue
    seen.add(word)
    words.push(word)
  }
  return words.join(locale === 'en-US' ? ', ' : '、')
}
