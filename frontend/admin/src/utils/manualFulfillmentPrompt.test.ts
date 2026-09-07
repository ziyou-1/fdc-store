import { describe, expect, it } from 'vitest'
import type { AdminOrder, AdminOrderItem } from '@/api/types'
import {
  countManualFulfillmentPrompts,
  firstAwaitingManualFulfillmentOrder,
  isAwaitingManualFulfillment,
  orderNeedsManualFulfillmentPrompt,
  submissionHasValue,
} from './manualFulfillmentPrompt'

const item = (overrides: Partial<AdminOrderItem> = {}): AdminOrderItem =>
  ({
    id: 1,
    order_id: 1,
    product_id: 1,
    sku_id: 1,
    title: { 'zh-CN': '激活码' },
    quantity: 1,
    original_unit_price: 1,
    unit_price: 1,
    cost_price: 0,
    original_total_price: 1,
    total_price: 1,
    fulfillment_type: 'manual',
    created_at: '',
    ...overrides,
  }) as AdminOrderItem

const order = (overrides: Partial<AdminOrder> = {}): AdminOrder =>
  ({
    id: 1,
    order_no: 'DJ-1',
    user_id: 1,
    status: 'fulfilling',
    currency: 'CNY',
    original_amount: 1,
    discount_amount: 0,
    promotion_discount_amount: 0,
    total_amount: 1,
    wallet_paid_amount: 0,
    online_paid_amount: 1,
    refunded_amount: 0,
    created_at: '',
    updated_at: '',
    items: [item({ manual_form_submission: { activation_email: 'a@b.com' } })],
    ...overrides,
  }) as AdminOrder

describe('manual fulfillment prompt', () => {
  it('treats empty or blank submissions as not filled', () => {
    expect(submissionHasValue(undefined)).toBe(false)
    expect(submissionHasValue({})).toBe(false)
    expect(submissionHasValue({ activation_email: '  ' })).toBe(false)
    expect(submissionHasValue({ activation_email: 'a@b.com' })).toBe(true)
  })

  it('flags a fulfilling leaf only after the buyer submitted values', () => {
    expect(isAwaitingManualFulfillment(order())).toBe(true)
    expect(isAwaitingManualFulfillment(order({ items: [item({ manual_form_submission: {} })] }))).toBe(false)
    expect(isAwaitingManualFulfillment(order({ status: 'completed' }))).toBe(false)
    expect(
      isAwaitingManualFulfillment(
        order({
          fulfillment: { id: 1, order_id: 1, type: 'manual', status: 'delivered', content: '', created_at: '', updated_at: '' },
        }),
      ),
    ).toBe(false)
  })

  it('walks parent → child so the list row lights up after after-payment email submit', () => {
    const child = order({ id: 72, order_no: 'DJ-1-01', parent_id: 71 })
    const parent = order({
      id: 71,
      order_no: 'DJ-1',
      items: [item({ manual_form_submission: { activation_email: 'a@b.com' } })],
      children: [child],
    })
    expect(orderNeedsManualFulfillmentPrompt(parent)).toBe(true)
    expect(firstAwaitingManualFulfillmentOrder(parent)?.id).toBe(72)
    expect(countManualFulfillmentPrompts([parent, order({ id: 69, status: 'completed' })])).toBe(1)
  })
})
