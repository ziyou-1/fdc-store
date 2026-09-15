export type PaymentResetReason = 'generic' | 'route_change' | 'change_payment_method'

export interface PaymentResetPolicy {
  resumeLatestPayment: boolean
  clearSelectedChannel: boolean
  stopActivePaymentWatch: boolean
}

export interface CachedPaymentRestorePolicy {
  startActivePaymentWatch: boolean
  autoOpenPayLink: boolean
}

export type PaymentPresentationMode = 'qr' | 'redirect'
export type PaymentLinkNavigationTarget = 'current-tab' | 'new-window'
export type PaymentInteractionLabelKey =
  | 'payment.modeQr'
  | 'payment.modeRedirect'
  | 'payment.modeWap'
  | 'payment.modePage'
export type PaymentResultTitleKey =
  | 'payment.resultTitle'
  | 'payment.resultRedirectTitle'
  | 'payment.modeWap'
  | 'payment.modePage'

const REDIRECT_PAYMENT_INTERACTION_MODES = new Set(['redirect', 'wap', 'page'])
const normalizeInteractionMode = (mode: unknown) => String(mode || '').trim().toLowerCase()

export const isCustomerSurchargePayment = (payment?: { fee_policy?: unknown } | null) => {
  const policy = String(payment?.fee_policy || '').trim().toLowerCase()
  return policy === 'customer_surcharge' || policy === 'legacy_customer_surcharge'
}

export const isRedirectPaymentInteractionMode = (mode: unknown) => {
  return REDIRECT_PAYMENT_INTERACTION_MODES.has(normalizeInteractionMode(mode))
}

export const resolvePaymentPresentationMode = (mode: unknown): PaymentPresentationMode => {
  return isRedirectPaymentInteractionMode(mode) ? 'redirect' : 'qr'
}

export const resolvePaymentLinkNavigationTarget = (_automatic: boolean): PaymentLinkNavigationTarget => {
  // 一律新窗口打开支付站，不整窗替换当前标签页。
  // 原先自动跳转用 current-tab 会把买家整窗带到 Payssion 这类第三方托管页，
  // 而那种页面只有一个二维码、没有返回入口，买家付完/放弃都回不到商城。
  // 改成 new-window 后我方 /pay 页始终留守（带返回首页按钮 + 轮询支付状态）：
  // 新窗成功则支付站在新标签打开、我方页仍在；被浏览器拦截则我方页显示「去支付」按钮，
  // 买家点一下（用户手势不会被拦）再打开。两种情况买家都不会被困在第三方页。
  return 'new-window'
}

export const resolvePaymentInteractionLabelKey = (mode: unknown): PaymentInteractionLabelKey | null => {
  switch (normalizeInteractionMode(mode)) {
    case 'qr':
      return 'payment.modeQr'
    case 'redirect':
      return 'payment.modeRedirect'
    case 'wap':
      return 'payment.modeWap'
    case 'page':
      return 'payment.modePage'
    default:
      return null
  }
}

export const resolvePaymentResultTitleKey = (mode: unknown): PaymentResultTitleKey => {
  switch (normalizeInteractionMode(mode)) {
    case 'wap':
      return 'payment.modeWap'
    case 'page':
      return 'payment.modePage'
    case 'redirect':
      return 'payment.resultRedirectTitle'
    default:
      return 'payment.resultTitle'
  }
}

export const getPaymentResetPolicy = (reason: PaymentResetReason = 'generic'): PaymentResetPolicy => {
  if (reason === 'change_payment_method') {
    return {
      resumeLatestPayment: false,
      clearSelectedChannel: true,
      stopActivePaymentWatch: true,
    }
  }

  return {
    resumeLatestPayment: true,
    clearSelectedChannel: false,
    stopActivePaymentWatch: false,
  }
}

export const getCachedPaymentRestorePolicy = (): CachedPaymentRestorePolicy => ({
  startActivePaymentWatch: true,
  autoOpenPayLink: false,
})

export const shouldAutoOpenPaymentLink = (payment?: { interaction_mode?: unknown; pay_url?: unknown; fee_policy?: unknown } | null) => {
  if (!payment || isCustomerSurchargePayment(payment)) return false
  const payURL = String(payment.pay_url || '').trim()
  return isRedirectPaymentInteractionMode(payment.interaction_mode) && payURL !== ''
}
