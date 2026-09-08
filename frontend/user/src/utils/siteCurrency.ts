// 站点币种的唯一取值口径。
//
// 这之前散在九处：useCart / useCheckout / useProduct / useProductDetail /
// GiftCardPanel / WalletPanel 各写各的，且一律硬编码 'CNY' 兜底。站点币种从
// CNY 改成 USD 再改成 IDR 之后，这些兜底就都是错的；而会员套餐页干脆没传币种，
// 300000.00 后面光秃秃什么都没有，买家不知道这是什么钱。

/** 站点配置里 currency 缺失或非法时的兜底币种。 */
export const FALLBACK_CURRENCY = 'USD'

/**
 * 从站点配置里取币种，归一化为大写三字母代码。
 * 取不到就回落 FALLBACK_CURRENCY——绝不返回空串，
 * 空串会让金额显示成裸数字，买家读不出这是哪国的钱。
 */
export const resolveSiteCurrency = (raw?: unknown): string => {
  const value = String(raw ?? '').trim().toUpperCase()
  return /^[A-Z]{3}$/.test(value) ? value : FALLBACK_CURRENCY
}

/**
 * 金额 + 币种的展示串。amount 为空返回 '-'；
 * 币种缺失时不再静默省略，而是回落站点兜底币种。
 */
export const formatAmountWithCurrency = (amount?: string | number | null, currency?: string): string => {
  if (amount === null || amount === undefined || String(amount).trim() === '') return '-'
  return `${amount} ${resolveSiteCurrency(currency)}`
}
