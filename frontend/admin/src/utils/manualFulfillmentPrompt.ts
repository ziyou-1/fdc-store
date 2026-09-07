import type { AdminOrder, AdminOrderItem } from '@/api/types'

const AWAITING_STATUSES = new Set(['paid', 'fulfilling'])

const valueIsFilled = (value: unknown): boolean => {
  if (value == null) return false
  if (typeof value === 'string') return value.trim() !== ''
  if (Array.isArray(value)) return value.some(valueIsFilled)
  if (typeof value === 'object') {
    return Object.values(value as Record<string, unknown>).some(valueIsFilled)
  }
  return true
}

export const submissionHasValue = (submission: unknown): boolean => {
  if (!submission || typeof submission !== 'object' || Array.isArray(submission)) {
    return false
  }
  return Object.values(submission as Record<string, unknown>).some(valueIsFilled)
}

export const itemHasManualSubmission = (item: Pick<AdminOrderItem, 'fulfillment_type' | 'manual_form_submission'>): boolean =>
  item.fulfillment_type === 'manual' && submissionHasValue(item.manual_form_submission)

export const isAwaitingManualFulfillment = (
  order: Pick<AdminOrder, 'status' | 'fulfillment' | 'items'> | null | undefined,
): boolean => {
  if (!order || !AWAITING_STATUSES.has(order.status)) return false
  if (order.fulfillment && order.fulfillment.status !== 'pending') return false
  return (order.items ?? []).some(itemHasManualSubmission)
}

export const orderNeedsManualFulfillmentPrompt = (order: AdminOrder | null | undefined): boolean => {
  if (!order) return false
  if (order.children?.length) {
    if (order.children.some(orderNeedsManualFulfillmentPrompt)) return true
  }
  return isAwaitingManualFulfillment(order)
}

export const firstAwaitingManualFulfillmentOrder = (order: AdminOrder | null | undefined): AdminOrder | null => {
  if (!order) return null
  if (order.children?.length) {
    for (const child of order.children) {
      const found = firstAwaitingManualFulfillmentOrder(child)
      if (found) return found
    }
    if (isAwaitingManualFulfillment(order)) {
      return order.children.find((child) => AWAITING_STATUSES.has(child.status) && !child.fulfillment) ?? null
    }
    return null
  }
  return isAwaitingManualFulfillment(order) ? order : null
}

export const countManualFulfillmentPrompts = (orders: AdminOrder[] | null | undefined): number =>
  (orders ?? []).filter(orderNeedsManualFulfillmentPrompt).length
