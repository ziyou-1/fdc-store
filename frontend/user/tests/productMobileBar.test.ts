import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, join } from 'node:path'

const here = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(
  join(here, '../src/components/product/ProductMobileBar.vue'),
  'utf8',
)

// 只取 <template> 部分，避免注释里出现的词误伤断言。
const template = source.slice(source.indexOf('<template>'), source.indexOf('</template>'))
const withoutComments = template.replace(/<!--[\s\S]*?-->/g, '')

// 取按钮行（flex items-stretch 那个容器之后、</div> 之前）的片段
const buttonRow = withoutComments.slice(withoutComments.indexOf('flex items-stretch'))

// 实测数据（375px 视口）：价格与按钮挤同一行时，每个按钮只剩 70px、
// 可用文字宽 52px；印尼语「Masukkan Keranjang」需要 127px，只显示得下四成。
// 分成两行后按钮拿到 112px，三种文案都完整。
// 这条守卫钉住「分行」这个结构，防止有人改回一行挤三个按钮。
test('价格与购买按钮必须分行，不能挤在同一个 flex 行里', () => {
  // 价格块与按钮行各自独立，中间不是同一个 flex 容器
  const priceRowIndex = withoutComments.indexOf('mb-1.5 min-w-0')
  const buttonRowIndex = withoutComments.indexOf('flex items-stretch')
  assert.ok(priceRowIndex > -1, '价格应当独占一行（mb-1.5 min-w-0）')
  assert.ok(buttonRowIndex > -1, '按钮应当在自己的 flex 行里（flex items-stretch）')
  assert.ok(priceRowIndex < buttonRowIndex, '价格行应排在按钮行之前')
})

test('购买按钮不得截断文字', () => {
  assert.ok(
    !buttonRow.includes('truncate'),
    'truncate 会把「加入购物车」「Masukkan Keranjang」截成半截，按钮文案必须完整可读',
  )
})

test('购买按钮必须允许换行并自适应高度', () => {
  // 长文案（印尼语）靠换行容纳，所以不能锁死单行与固定高度
  assert.ok(buttonRow.includes('whitespace-normal'), '按钮需要 whitespace-normal 才能换行')
  assert.ok(buttonRow.includes('min-h-11'), '按钮应使用 min-h 而不是固定 h，换行后才撑得开')
  // 注意边界：\bh-11\b 会在 min-h-11 内部命中（- 也算单词边界），
  // 所以必须要求 h-11 前面是空格或引号，才只匹配「独立的固定高度」。
  assert.ok(
    !/(?:^|[\s"])h-11(?![\w-])/.test(buttonRow),
    '固定 h-11 会让换行后的第二行被裁掉，应使用 min-h-11 + h-auto',
  )
})
