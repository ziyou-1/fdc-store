import { userOrderAPI } from '../api'
import { getAffiliateCode, getAffiliateVisitorKey } from './affiliate'

export async function createSkipPaymentOrder(item: {
  productId: number
  skuId?: number
  quantity: number
  fulfillmentType?: string
}) {
  const response = await userOrderAPI.createAndPay({
    items: [{
      product_id: item.productId,
      sku_id: item.skuId || undefined,
      quantity: item.quantity,
      fulfillment_type: item.fulfillmentType || undefined,
    }],
    affiliate_code: getAffiliateCode() || undefined,
    affiliate_visitor_key: getAffiliateVisitorKey() || undefined,
  })
  const data = response.data?.data
  if (!data?.order_no) {
    throw new Error('create order failed')
  }
  if (data.payment_error) {
    throw new Error(String(data.payment_error))
  }
  if (!data.order_paid) {
    throw new Error('order not paid')
  }
  return data
}
