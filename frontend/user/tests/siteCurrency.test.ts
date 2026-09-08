import test from 'node:test'
import assert from 'node:assert/strict'
import { resolveSiteCurrency, formatAmountWithCurrency, FALLBACK_CURRENCY } from '../src/utils/siteCurrency.ts'

test('站点币种归一化为大写三字母', () => {
  assert.equal(resolveSiteCurrency('idr'), 'IDR')
  assert.equal(resolveSiteCurrency(' USD '), 'USD')
  assert.equal(resolveSiteCurrency('CNY'), 'CNY')
})

// 取不到币种时绝不能返回空串——空串会让金额显示成裸数字，
// 买家不知道 300000.00 是什么钱。会员套餐页原先就是这个毛病。
test('缺失或非法时回落兜底币种，不返回空串', () => {
  for (const bad of [undefined, null, '', '   ', 'X', 'ABCD', '123', 42]) {
    const got = resolveSiteCurrency(bad as unknown)
    assert.equal(got, FALLBACK_CURRENCY, `${String(bad)} 应回落 ${FALLBACK_CURRENCY}`)
    assert.ok(got.length === 3)
  }
})

test('金额与币种拼接', () => {
  assert.equal(formatAmountWithCurrency('300000.00', 'IDR'), '300000.00 IDR')
  assert.equal(formatAmountWithCurrency(98, 'usd'), '98 USD')
})

test('金额为空返回占位符而不是拼一个孤零零的币种', () => {
  assert.equal(formatAmountWithCurrency(undefined, 'IDR'), '-')
  assert.equal(formatAmountWithCurrency('', 'IDR'), '-')
  assert.equal(formatAmountWithCurrency(null, 'IDR'), '-')
})

test('金额有值但币种缺失时仍带上兜底币种', () => {
  assert.equal(formatAmountWithCurrency('12.00'), `12.00 ${FALLBACK_CURRENCY}`)
})
