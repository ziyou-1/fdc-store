import test from 'node:test'
import assert from 'node:assert/strict'
import {
  collectContentTutorialBlocks,
  shouldAutoOpenContentTutorial,
  toTutorialHtml,
} from '../src/utils/contentTutorial.ts'

const localize = (value: unknown) => {
  if (!value || typeof value !== 'object') return ''
  return String((value as Record<string, string>)['en-US'] || (value as Record<string, string>)['zh-CN'] || '')
}

const contentOrder = {
  children: [{
    fulfillment: { type: 'content', status: 'delivered' },
    items: [{
      fulfillment_type: 'content',
      title: { 'en-US': 'Valorant Points Guide' },
      instructions: { 'en-US': 'Follow the tutorial link.\nVideo: https://example.com/valorant.mp4' },
    }],
  }],
}

test('type-3 paid order must auto-open the tutorial', () => {
  assert.equal(shouldAutoOpenContentTutorial(contentOrder, localize), true)
  const blocks = collectContentTutorialBlocks(contentOrder, localize)
  assert.equal(blocks.length, 1)
  assert.match(blocks[0].html, /<video/)
  assert.match(blocks[0].html, /https:\/\/example.com\/valorant\.mp4/)
})

test('auto-card orders do not steal the type-3 popup', () => {
  assert.equal(shouldAutoOpenContentTutorial({
    fulfillment: { type: 'auto', status: 'delivered' },
    items: [{
      fulfillment_type: 'auto',
      instructions: { 'en-US': 'Login then follow https://example.com/guide' },
    }],
  }, localize), false)
})

test('plain tutorial urls become links or media', () => {
  const html = toTutorialHtml('Shot: https://example.com/a.png\nMore: https://example.com/help')
  assert.match(html, /<img/)
  assert.match(html, /<a /)
})
