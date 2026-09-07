import test from 'node:test'
import assert from 'node:assert/strict'
import { parseFulfillmentCredentials, parseSecretLine } from '../src/utils/fulfillmentCredentials.ts'

test('parseSecretLine splits account----password', () => {
  assert.deepEqual(parseSecretLine('user@mail.com----s3cret'), {
    account: 'user@mail.com',
    password: 's3cret',
  })
})

test('parseSecretLine splits compact account:password', () => {
  assert.deepEqual(parseSecretLine('demo:passw0rd'), {
    account: 'demo',
    password: 'passw0rd',
  })
})

test('parseSecretLine rejects urls, notes, and bare codes', () => {
  assert.equal(parseSecretLine('https://example.com/watch'), null)
  assert.equal(parseSecretLine('SECRET-SKU-1001'), null)
  assert.equal(parseSecretLine('note: please save this'), null)
  assert.equal(parseSecretLine(''), null)
})

test('parseFulfillmentCredentials pairs labeled delivery entries', () => {
  const view = parseFulfillmentCredentials({
    delivery_data: {
      note: 'keep safe',
      entries: [
        { key: 'account', value: 'acc-1' },
        { key: 'password', value: 'pw-1' },
        { key: 'remark', value: 'slot A' },
      ],
    },
  })
  assert.deepEqual(view.credentials, [{ account: 'acc-1', password: 'pw-1' }])
  assert.deepEqual(view.extraLines, ['keep safe', 'remark: slot A'])
})

test('parseFulfillmentCredentials reads auto payload lines', () => {
  const view = parseFulfillmentCredentials({
    payload: 'a----1\nSECRET-ONLY\nb----2\n',
  })
  assert.deepEqual(view.credentials, [
    { account: 'a', password: '1' },
    { account: 'b', password: '2' },
  ])
  assert.deepEqual(view.extraLines, ['SECRET-ONLY'])
})
