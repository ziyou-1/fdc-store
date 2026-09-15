export const disabledFeatures = {
  // socialLogin 与 reseller 的后端在 config.yml 里仍是 enabled: false
  // （telegram_auth / google_auth / reseller），前端放开只会露出点不动的入口，
  // 因此保持关闭，等后端配置就绪后再一并开启。
  reseller: true,
  socialLogin: true,

  // buyerApi 停用：买家端不提供 API 对接入口。
  // 布尔值只管藏侧栏入口，堵 URL 要靠下面的 DISABLED_PATH_PREFIXES。
  buyerApi: true,
  // giftCards 停用：买家端不提供礼品卡兑换。
  // 后台管理入口本来就是关的，两端现在一致。
  giftCards: true,
  wholesale: false,
  // 现金分销（佣金/提现）下线：改成只做积分推广。买家端不再显示佣金面板，
  // 后端也把 affiliate.enabled 关掉停止佣金累计。积分推广是另一套（points），不受影响。
  cashAffiliate: true,
  buyerWallet: false,
  vaultTemplate: false,
  coupons: false,
  thresholdVip: false,
} as const

const DISABLED_PATH_PREFIXES = [
  // 仅保留后端未就绪的入口
  '/me/reseller',
  '/reseller',
  '/auth/telegram',
  '/auth/google',
  // 买家端 API 对接已下线
  '/me/api',
  // 礼品卡兑换已下线
  '/me/gift-cards',
  // 现金分销下线，只保留积分推广
  '/me/affiliate',
] as const

const DISABLED_EXACT_PATHS = [] as const

export const disabledPersonalSections = ['reseller', 'api', 'giftCard', 'affiliate'] as const

export function normalizeFeaturePath(path: string): string {
  const trimmed = path.trim()
  if (trimmed === '') return '/'
  const withSlash = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  if (withSlash.length > 1 && withSlash.endsWith('/')) return withSlash.slice(0, -1)
  return withSlash
}

export function isDisabledFeaturePath(path: string): boolean {
  const normalized = normalizeFeaturePath(path)
  if ((DISABLED_EXACT_PATHS as readonly string[]).includes(normalized)) return true
  return DISABLED_PATH_PREFIXES.some((prefix) => normalized === prefix || normalized.startsWith(`${prefix}/`))
}

export function isDisabledPersonalSection(section: string): boolean {
  return (disabledPersonalSections as readonly string[]).includes(section)
}
