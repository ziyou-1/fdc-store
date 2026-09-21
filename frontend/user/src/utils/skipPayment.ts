export function isSkipPaymentEnabled(config: { skip_payment?: unknown } | null | undefined): boolean {
  return config?.skip_payment === true
}

export function requiresCheckoutTimeManualForm(product: {
  manualFormTiming?: unknown
  manualFormSchema?: unknown
  // 所选规格的交付方式：只有人工/上游代销才需要下单时采集表单（交付方式已下沉到 SKU）。
  fulfillmentType?: unknown
} | null | undefined): boolean {
  if (!product) return false
  const fulfillmentType = String(product.fulfillmentType || '').trim()
  if (fulfillmentType && fulfillmentType !== 'manual' && fulfillmentType !== 'upstream') return false
  if (String(product.manualFormTiming || '').trim() === 'after_payment') return false
  const raw = product.manualFormSchema as { fields?: unknown } | null | undefined
  return Array.isArray(raw?.fields) && raw.fields.length > 0
}
