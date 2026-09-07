import test from 'node:test'
import assert from 'node:assert/strict'
import { referralInviteURL } from '../src/utils/referralInvite.ts'

test('invite URL uses aff query on origin', () => {
  assert.equal(
    referralInviteURL('http://127.0.0.1:5175', 'QZ7MD9LD'),
    'http://127.0.0.1:5175/?aff=QZ7MD9LD',
  )
})

test('trailing slash on origin is stripped once', () => {
  assert.equal(
    referralInviteURL('http://127.0.0.1:5175/', 'ABC'),
    'http://127.0.0.1:5175/?aff=ABC',
  )
})
