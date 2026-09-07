export const disabledFeatures = {
  giftCards: true,
  orderRisk: true,
  blog: true,
  coupons: true,
  promotions: true,
  wholesale: true,
  cashAffiliate: false,
  reseller: true,
  reconciliation: true,
  apiCredentials: true,
  telegramBot: true,
  socialLogin: true,
  vaultTemplate: true,
  walletConfig: true,
  thresholdVip: true,
} as const

const DISABLED_PATH_PREFIXES = [
  '/gift-cards',
  '/order-risk-control',
  '/posts/blog',
  '/posts/categories',
  '/coupons',
  '/promotions',
  '/wholesale-prices',
  '/resellers',
  '/reconciliation',
  '/api-credentials',
  '/telegram-bot',
  '/wallet-config',
] as const

export const disabledSettingsTabs = ['template', 'telegram', 'google'] as const

export function normalizeFeaturePath(path: string): string {
  const trimmed = path.trim()
  if (trimmed === '') return '/'
  const withSlash = trimmed.startsWith('/') ? trimmed : `/${trimmed}`
  if (withSlash.length > 1 && withSlash.endsWith('/')) return withSlash.slice(0, -1)
  return withSlash
}

export function isDisabledFeaturePath(path: string): boolean {
  const normalized = normalizeFeaturePath(path)
  return DISABLED_PATH_PREFIXES.some((prefix) => normalized === prefix || normalized.startsWith(`${prefix}/`))
}

export function isDisabledSettingsTab(tab: string): boolean {
  return (disabledSettingsTabs as readonly string[]).includes(tab)
}
