import test from 'node:test'
import assert from 'node:assert/strict'
import { isSkipPaymentEnabled, requiresCheckoutTimeManualForm } from '../src/utils/skipPayment.ts'

test('skip payment flag is explicit true only', () => {
  assert.equal(isSkipPaymentEnabled({ skip_payment: true }), true)
  assert.equal(isSkipPaymentEnabled({ skip_payment: false }), false)
  assert.equal(isSkipPaymentEnabled({}), false)
  assert.equal(isSkipPaymentEnabled(null), false)
})

test('after-payment email form does not block buy-now', () => {
  assert.equal(requiresCheckoutTimeManualForm({
    manualFormTiming: 'after_payment',
    manualFormSchema: { fields: [{ key: 'email', type: 'email', required: true }] },
  }), false)
})

test('checkout-time form still requires checkout', () => {
  assert.equal(requiresCheckoutTimeManualForm({
    manualFormTiming: 'checkout',
    manualFormSchema: { fields: [{ key: 'email', type: 'email', required: true }] },
  }), true)
})
