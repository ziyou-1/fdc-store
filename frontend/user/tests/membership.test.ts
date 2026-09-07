import test from 'node:test'
import assert from 'node:assert/strict'
import { discountOffN, purchasedMembershipAmount } from '../src/utils/membership.ts'

test('english discountOff n is percent off, not rate/10', () => {
  assert.equal(discountOffN(90, 'en-US'), 10)
  assert.equal(discountOffN(85, 'en-US'), 15)
  assert.equal(discountOffN(80, 'en-US'), 20)
})

test('chinese discountOff n is 折', () => {
  assert.equal(discountOffN(90, 'zh-CN'), 9)
  assert.equal(discountOffN(85, 'zh-TW'), 8.5)
  assert.equal(discountOffN(80, 'zh-CN'), 8)
})

test('purchased membership applies rate to listing price', () => {
  const profile = {
    membership_discount_rate: 90,
    membership_expires_at: '2099-01-01T00:00:00Z',
  }
  assert.equal(purchasedMembershipAmount('10.00', profile, Date.parse('2026-09-03T00:00:00Z')), '9.00')
  assert.equal(purchasedMembershipAmount('9.90', profile, Date.parse('2026-09-03T00:00:00Z')), '8.91')
})

test('purchased membership does not apply when expired or inactive', () => {
  assert.equal(
    purchasedMembershipAmount('10.00', {
      membership_discount_rate: 90,
      membership_expires_at: '2020-01-01T00:00:00Z',
    }, Date.parse('2026-09-03T00:00:00Z')),
    null,
  )
  assert.equal(purchasedMembershipAmount('10.00', null), null)
})
