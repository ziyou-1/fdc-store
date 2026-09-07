const HTML_TAG = /<[a-z][\s\S]*>/i
const VIDEO_URL = /\.(mp4|webm|ogg)(?:[?#].*)?$/i
const IMAGE_URL = /\.(png|jpe?g|gif|webp|avif)(?:[?#].*)?$/i

export type ContentTutorialBlock = {
  title: string
  html: string
}

type Localize = (value: unknown) => string

const escapeText = (value: string) =>
  value.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')

const wrapUrl = (url: string) => {
  const safe = escapeText(url)
  if (VIDEO_URL.test(url)) {
    return `<video controls preload="metadata" src="${safe}"></video>`
  }
  if (IMAGE_URL.test(url)) {
    return `<img src="${safe}" alt="">`
  }
  return `<a href="${safe}" target="_blank" rel="noopener noreferrer">${safe}</a>`
}

export const toTutorialHtml = (raw: string): string => {
  const text = String(raw || '').trim()
  if (!text) return ''
  if (HTML_TAG.test(text)) {
    return text
  }
  return escapeText(text)
    .replace(/https?:\/\/[^\s&<]+/gi, (url) => wrapUrl(url.replace(/&amp;/g, '&')))
    .replace(/\r\n|\n|\r/g, '<br>')
}

const fulfillmentTargets = (order: any): any[] => {
  if (!order) return []
  if (Array.isArray(order.children) && order.children.length > 0) {
    return order.children
  }
  return [order]
}

export const collectContentTutorialBlocks = (order: any, localize: Localize): ContentTutorialBlock[] => {
  const seen = new Set<string>()
  const blocks: ContentTutorialBlock[] = []
  for (const target of fulfillmentTargets(order)) {
    const fulfillment = target?.fulfillment
    if (!fulfillment || String(fulfillment.status || '') !== 'delivered') continue
    if (String(fulfillment.type || '').trim() !== 'content') continue
    const items = Array.isArray(target.items) ? target.items : []
    for (const item of items) {
      const raw = String(localize(item?.instructions) || '').trim()
      if (!raw) continue
      const html = toTutorialHtml(raw)
      if (!html || seen.has(html)) continue
      seen.add(html)
      blocks.push({
        title: String(localize(item?.title) || ''),
        html,
      })
    }
  }
  return blocks
}

export const shouldAutoOpenContentTutorial = (order: any, localize: Localize): boolean =>
  collectContentTutorialBlocks(order, localize).length > 0
