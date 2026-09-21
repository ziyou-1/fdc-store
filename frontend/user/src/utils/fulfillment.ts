type TranslateFn = (key: string, params?: Record<string, any>) => string

// 交付方式已下沉到 SKU：所选规格自带 fulfillment_type 就用它，否则回落商品级默认值。
// 商品详情/快速购买/购物车快照/库存口径都应以这个结果为准。
export const resolveSkuFulfillmentType = (product: any, sku?: any): string => {
  const own = String(sku?.fulfillment_type || '').trim()
  if (own) return own
  return String(product?.fulfillment_type || '').trim() || 'manual'
}

export type FulfillmentLabelScope = 'orderDetail'

export const fulfillmentTypeLabel = (t: TranslateFn, type?: string, scope: FulfillmentLabelScope = 'orderDetail') => {
  if (!type) return '-'
  const baseKey = `${scope}.fulfillmentTypes`
  const map: Record<string, string> = {
    manual: t(`${baseKey}.manual`),
    auto: t(`${baseKey}.auto`),
    content: t(`${baseKey}.content`),
    upstream: t(`${baseKey}.manual`),
  }
  return map[type] || type
}

export const fulfillmentStatusLabel = (t: TranslateFn, status?: string, scope: FulfillmentLabelScope = 'orderDetail') => {
  if (!status) return '-'
  const baseKey = `${scope}.fulfillmentStatuses`
  const map: Record<string, string> = {
    pending: t(`${baseKey}.pending`),
    delivered: t(`${baseKey}.delivered`),
  }
  return map[status] || status
}
