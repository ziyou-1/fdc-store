import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

// 客服输入框是 rows=1 + min-h-9，可见内容高只有一行（20px）。
// 390px 宽的手机上（iPhone 12/13/14 的 CSS 宽度），输入框文字可用宽度实测 220px：
//   面板 100vw-2rem = 358，减 p-3 内边距 24，减附件按钮 36 + 发送按钮 36 + 两个 gap 16，
//   再减 textarea 自己的 px-3 = 24。
// 占位文字一旦超过一行就会被切掉，买家看到的是半截字——曾经四个语种全中。
const MAX_UNITS = 30

// 估算显示宽度：CJK / 全角标点按 2 个单位，其余按 1 个。
// 用单位数而不是字符数，是因为 14px 下一个汉字约 14px、一个拉丁字母约 7px，
// 只数字符会让中文过关、英文误报。
const displayUnits = (text: string): number => {
  let units = 0
  for (const ch of text) {
    const code = ch.codePointAt(0) ?? 0
    const wide =
      (code >= 0x1100 && code <= 0x115f) ||
      (code >= 0x2e80 && code <= 0xa4cf) ||
      (code >= 0xac00 && code <= 0xd7a3) ||
      (code >= 0xf900 && code <= 0xfaff) ||
      (code >= 0xfe30 && code <= 0xfe4f) ||
      (code >= 0xff00 && code <= 0xff60) ||
      (code >= 0xffe0 && code <= 0xffe6)
    units += wide ? 2 : 1
  }
  return units
}

const LOCALES = ['zh-CN', 'zh-TW', 'en-US', 'id-ID']

const placeholderOf = (locale: string): string => {
  const raw = readFileSync(new URL(`../src/i18n/locales/${locale}.json`, import.meta.url), 'utf8')
  return JSON.parse(raw).support.placeholder
}

test('客服输入框占位文字必须能在手机上放进一行', () => {
  for (const locale of LOCALES) {
    const text = placeholderOf(locale)
    const units = displayUnits(text)
    assert.ok(
      units <= MAX_UNITS,
      `${locale} 的 support.placeholder ${JSON.stringify(text)} 估算宽度 ${units} 单位，` +
        `超过 ${MAX_UNITS}，在 390px 宽的手机上会折行并被输入框切掉`,
    )
  }
})

test('四个语种都得有非空的占位文字', () => {
  for (const locale of LOCALES) {
    const text = placeholderOf(locale)
    assert.equal(typeof text, 'string')
    assert.ok(text.trim().length > 0, `${locale} 的 support.placeholder 不能为空`)
  }
})

test('宽度估算把汉字算成两个单位', () => {
  assert.equal(displayUnits('abc'), 3)
  assert.equal(displayUnits('问题'), 4)
  // 6 个汉字算 12，省略号 U+2026 是 ambiguous width，按窄的算 1
  assert.equal(displayUnits('说说你的问题…'), 13)
})
