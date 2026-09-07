import test from 'node:test'
import assert from 'node:assert/strict'
import { showLoginForgotPassword } from '../src/utils/loginForgotPassword.ts'

test('login shows forgot password when registration email verification is off', () => {
  assert.equal(showLoginForgotPassword({ email_verification_enabled: false }), true)
})

test('login shows forgot password when public config omitted the flag', () => {
  assert.equal(showLoginForgotPassword({}), true)
})
