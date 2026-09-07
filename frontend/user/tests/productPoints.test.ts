import test from 'node:test'
import assert from 'node:assert/strict'
import { canRedeemWithPoints, pointsPriceOf, redeemPointsCost } from '../src/utils/productPoints.ts'

test('cash-only product is not redeemable', () => {
  assert.equal(pointsPriceOf({ points_price: 0 }), 0)
  assert.equal(canRedeemWithPoints({ points_price: 0 }), false)
  assert.equal(redeemPointsCost({ points_price: 0 }, null, 1), 0)
})

test('product points_price 50 is redeemable', () => {
  assert.equal(pointsPriceOf({ points_price: 50 }), 50)
  assert.equal(canRedeemWithPoints({ points_price: 50 }), true)
  assert.equal(redeemPointsCost({ points_price: 50 }, null, 2), 100)
})

test('SKU points_price wins over product', () => {
  assert.equal(pointsPriceOf({ points_price: 0 }, { points_price: 50 }), 50)
  assert.equal(canRedeemWithPoints({ points_price: 0 }, { points_price: 50 }), true)
})
