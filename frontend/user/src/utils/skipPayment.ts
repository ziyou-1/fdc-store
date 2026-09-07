export function isSkipPaymentEnabled(config: { skip_payment?: unknown } | null | undefined): boolean {
  return config?.skip_payment === true
}

export function requiresCheckoutTimeManualForm(product: {
  manualFormTiming?: unknown
  manualFormSchema?: unknown
} | null | undefined): boolean {
  if (!product) return false
  if (String(product.manualFormTiming || '').trim() === 'after_payment') return false
  const raw = product.manualFormSchema as { fields?: unknown } | null | undefined
  return Array.isArray(raw?.fields) && raw.fields.length > 0
}
