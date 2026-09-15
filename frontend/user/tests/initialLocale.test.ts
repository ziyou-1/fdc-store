import test from 'node:test'
import assert from 'node:assert/strict'
import { DEFAULT_LOCALE, SUPPORTED_LOCALES, resolveInitialLocale } from '../src/utils/locale.ts'

test('首屏默认印尼语：买家没选过语言时不看浏览器语言', () => {
  assert.equal(DEFAULT_LOCALE, 'id-ID')
  for (const saved of [null, undefined, '', '   ']) {
    assert.equal(resolveInitialLocale(saved), 'id-ID')
  }
})

test('买家自己选过的语言会被记住', () => {
  for (const locale of SUPPORTED_LOCALES) {
    assert.equal(resolveInitialLocale(locale), locale)
  }
})

test('存了不支持的语言时回落站点默认语言，而不是中文', () => {
  for (const saved of ['fr-FR', 'ja-JP', 'zh', 'id', 'garbage']) {
    assert.equal(resolveInitialLocale(saved), 'id-ID')
  }
})

test('印尼语在支持列表里，且默认语言必须属于支持列表', () => {
  assert.ok((SUPPORTED_LOCALES as readonly string[]).includes('id-ID'))
  assert.ok((SUPPORTED_LOCALES as readonly string[]).includes(DEFAULT_LOCALE))
})
