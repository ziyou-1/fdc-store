// 首屏语言的判定规则。
//
// 单独抽出来是为了能被测到：i18n/index.ts 在模块加载时就会 createI18n、
// 读 localStorage 和 navigator，在 node --test 里跑不起来；而"首屏该显示哪种语言"
// 是这个站点反复出问题的地方，必须有测试钉住。

/** 站点支持的界面语言。顺序即语言切换器里的展示顺序。 */
export const SUPPORTED_LOCALES = ['zh-CN', 'zh-TW', 'en-US', 'id-ID'] as const

/** 站点默认语言。本站面向印尼市场。 */
export const DEFAULT_LOCALE = 'id-ID'

/**
 * 由"买家上次选过的语言"推出首屏该用哪种语言。
 *
 * 只认买家自己在页面上选过的值，其余一律用站点默认语言——刻意不看 navigator.language。
 * 按浏览器语言分流会让中文浏览器的访客直接落到中文界面，于是运营和客服自己看到的首屏
 * 和买家看到的不是同一个：印尼语这边缺文案、价格没换算，自己人永远发现不了。
 *
 * @param saved localStorage 里存的语言，没有就传 null
 */
export function resolveInitialLocale(saved: string | null | undefined): string {
  const value = String(saved ?? '').trim()
  return (SUPPORTED_LOCALES as readonly string[]).includes(value) ? value : DEFAULT_LOCALE
}
