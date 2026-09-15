import { createI18n } from 'vue-i18n'
// 默认语言（兜底语言）静态打包，其余语言按需动态加载。
// 本站面向印尼市场，默认语言即印尼语：它是绝大多数买家的首屏语言，
// 静态打包能省掉一次语言包往返；中文/英文改为按需懒加载。
import idID from './locales/id-ID.json'
import { DEFAULT_LOCALE, SUPPORTED_LOCALES, resolveInitialLocale } from '../utils/locale'

// 语言清单与默认语言的唯一真源在 utils/locale.ts —— 那里能被 node --test 跑到。
const supportedLocales = SUPPORTED_LOCALES as readonly string[]

// 非默认语言的懒加载器：切换语言时才下载对应语言包 chunk
const localeLoaders: Record<string, () => Promise<{ default: Record<string, unknown> }>> = {
    'zh-CN': () => import('./locales/zh-CN.json'),
    'zh-TW': () => import('./locales/zh-TW.json'),
    'en-US': () => import('./locales/en-US.json'),
}

const loadedLocales = new Set(['id-ID'])

export { DEFAULT_LOCALE }

/** 首屏语言。判定规则见 utils/locale.ts 的 resolveInitialLocale。 */
export function detectLocale(): string {
    return resolveInitialLocale(localStorage.getItem('locale'))
}

const i18n = createI18n({
    legacy: false,
    locale: DEFAULT_LOCALE,
    fallbackLocale: DEFAULT_LOCALE,
    // 键断言为 string，避免 locale 类型被收窄为 "id-ID" 字面量（其余语言在运行时动态注入）
    messages: { 'id-ID': idID } as Record<string, typeof idID>,
})

async function loadLocaleMessages(locale: string): Promise<void> {
    if (loadedLocales.has(locale)) return
    const loader = localeLoaders[locale]
    if (!loader) return
    const messages = await loader()
    i18n.global.setLocaleMessage(locale, (messages.default ?? messages) as any)
    loadedLocales.add(locale)
}

// 记录最新一次切换请求，防止快速连续切换时慢加载的旧请求覆盖新选择
let pendingLocale = ''

// 切换语言：先确保语言包加载完成再更新 locale，避免闪现缺失文案。
// 语言包加载失败时保持当前语言不变，避免整页文案退化为 key。
export async function setI18nLocale(locale: string): Promise<void> {
    if (!supportedLocales.includes(locale)) return
    pendingLocale = locale
    try {
        await loadLocaleMessages(locale)
    } catch (error) {
        console.error(`Failed to load locale messages: ${locale}`, error)
        return
    }
    if (pendingLocale !== locale) return
    i18n.global.locale.value = locale
}

// 空闲时预取其余语言包：让语言切换即时生效，也避免部署更新后旧页面
// 再切语言时请求已失效的旧 chunk（预取时已进入浏览器缓存）
export function warmupLocaleMessages(): void {
    if (typeof window === 'undefined') return

    const prefetch = () => {
        for (const locale of Object.keys(localeLoaders)) {
            void loadLocaleMessages(locale).catch(() => {
                // 预取失败静默忽略，实际切换时会重试并走 setI18nLocale 的错误处理
            })
        }
    }

    if ('requestIdleCallback' in window && typeof window.requestIdleCallback === 'function') {
        window.requestIdleCallback(prefetch, { timeout: 3000 })
        return
    }
    window.setTimeout(prefetch, 1000)
}

export default i18n
