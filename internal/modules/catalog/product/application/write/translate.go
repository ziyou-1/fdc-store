package productwrite

import (
	"strings"
	"sync"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
)

// Translator 是商品补译需要的最小能力。
//
// 声明在这里而不是 import support 模块的具体类型，是为了让 catalog 只描述
// "我需要什么"——仓库里 catalog/mapping/contract/ports.go 用的就是这个形状。
// 具体实现（DeepL 的热换槽位）由装配层塞进来。
//
// nil 表示没有翻译能力，补译整段跳过；这是常态而不是异常：
// 运营可能压根没配 DeepL 密钥，也可能主动把翻译关了。
type Translator interface {
	// TranslateBatch 把多条文本译成 targetLocale，返回与入参等长同序的译文。
	//
	// 入参用的是站点 locale（zh-CN / id-ID ...），不是翻译服务自己的语言码——
	// 那层映射归装配层的适配器管，catalog 不该知道 DeepL 把简体中文叫 ZH-HANS。
	// richText 为 true 表示内容是 HTML，需要按标签处理而不是纯文本。
	TranslateBatch(texts []string, targetLocale, sourceLocale string, richText bool) ([]string, error)
}

// localeFieldKind 决定一个多语言字段该怎么判空、怎么送去翻译。
type localeFieldKind int

const (
	// 纯文本：标题、描述、SEO 关键词等。
	localeFieldPlain localeFieldKind = iota
	// 富文本：商品详情与交付说明，由 tiptap 产出 HTML。
	// 必须带 tag_handling=html，否则 <a href> 里的 URL 会被当正文翻掉。
	localeFieldRichText
)

// isBlankLocaleValue 判断某个语言档位算不算"空"。
//
// 三种空要一起认，少认一种就会漏翻或白翻：
//  1. 键不存在——spec_values 与人工表单标签走的是这条，前端会丢弃空值再提交；
//  2. 值是空串或全空白——title/description 走这条，前端提交的是"四个键全在、值可能为空串"，
//     用"键是否存在"判断会永远认为四语齐全，一句都不翻；
//  3. 富文本的空壳——tiptap 里点进去打字再删光会留下 <p></p> 或 <p><br></p>，
//     肉眼看是空的，按字符串判非空就会把空壳当成"运营已经填过"而跳过。
func isBlankLocaleValue(raw map[string]interface{}, locale string, kind localeFieldKind) bool {
	value, exists := raw[locale]
	if !exists || value == nil {
		return true
	}
	text, ok := value.(string)
	if !ok {
		return true
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return true
	}
	if kind == localeFieldRichText && isEmptyRichText(trimmed) {
		return true
	}
	return false
}

// isEmptyRichText 识别富文本编辑器留下的空壳。
func isEmptyRichText(trimmed string) bool {
	collapsed := strings.ToLower(strings.Join(strings.Fields(trimmed), ""))
	switch collapsed {
	case "<p></p>", "<p><br></p>", "<p><br/></p>", "<p><br /></p>", "<br>", "<br/>":
		return true
	}
	return false
}

// localeValue 取某个语言档位的可用文本，取不到返回空串。
func localeValue(raw map[string]interface{}, locale string, kind localeFieldKind) string {
	if isBlankLocaleValue(raw, locale, kind) {
		return ""
	}
	text, _ := raw[locale].(string)
	return strings.TrimSpace(text)
}

// resolveSourceLocale 挑一个非空的语言档位作为翻译源。
//
// 优先简体中文（运营的实际录入语言），中文也空就按站点语言顺序找第一个非空的——
// 这样运营改用英文录入时补译一样能工作，不必回来改代码。
func resolveSourceLocale(raw map[string]interface{}, kind localeFieldKind) string {
	if !isBlankLocaleValue(raw, constants.LocaleZhCN, kind) {
		return constants.LocaleZhCN
	}
	for _, locale := range constants.SupportedLocales {
		if !isBlankLocaleValue(raw, locale, kind) {
			return locale
		}
	}
	return ""
}

// localeGap 描述"某个字段的某门语言缺了，要用哪段原文去补"。
type localeGap struct {
	field        map[string]interface{}
	locale       string
	source       string
	sourceLocale string
	kind         localeFieldKind
}

// translatableField 是待补译的一个多语言字段。
type translatableField struct {
	values map[string]interface{}
	kind   localeFieldKind
}

// fillMissingLocales 把每个字段缺失的语言用机器翻译补上。
//
// 语义有三条，缺一条都会出事：
//
//  1. 只补空缺，绝不覆盖已有值。运营人工校对过的译文不能被下一次保存冲掉，
//     否则"可以改译文"就是假的。
//  2. fail-open。DeepL 没配、报错、超时，一律只记结果不阻断——
//     商品保存绝不能被第三方 API 挡住。
//  3. 绝不能在数据库事务里调用。这个站点默认跑 SQLite 且最大连接数是 1，
//     事务期间那唯一一条连接被独占，DeepL 一慢就是整个进程卡死；
//     而 SQLite 的 busy_timeout 只有 5 秒，比 DeepL 的 10 秒超时还短。
//     调用点必须在 WithinTransaction 之前。
//
// 返回补上了多少个语言档位，供调用方记日志。
func fillMissingLocales(translator Translator, fields []translatableField) int {
	if translator == nil || len(fields) == 0 {
		return 0
	}

	// 按 (目标语言, 文本类型) 归组：tag_handling 是整个请求级的参数，
	// 富文本和纯文本不能混在同一个请求里，混了要么标签被翻坏、
	// 要么纯文本被当 HTML 处理。
	type groupKey struct {
		locale string
		kind   localeFieldKind
	}
	groups := map[groupKey][]localeGap{}

	for _, field := range fields {
		if field.values == nil {
			continue
		}
		source := resolveSourceLocale(field.values, field.kind)
		if source == "" {
			// 一门语言都没填，没有原文可译
			continue
		}
		sourceText := localeValue(field.values, source, field.kind)
		for _, locale := range constants.SupportedLocales {
			if locale == source || !isBlankLocaleValue(field.values, locale, field.kind) {
				continue
			}
			key := groupKey{locale: locale, kind: field.kind}
			groups[key] = append(groups[key], localeGap{
				field:        field.values,
				locale:       locale,
				source:       sourceText,
				sourceLocale: source,
				kind:         field.kind,
			})
		}
	}
	if len(groups) == 0 {
		return 0
	}

	// 每组一次请求，各组并发发出去。
	// 串行的话最坏是 6 次 × 10 秒超时 = 60 秒，前端 10 秒就把请求砍了。
	type groupResult struct {
		gaps   []localeGap
		texts  []string
		failed bool
	}
	results := make([]groupResult, 0, len(groups))
	var mu sync.Mutex
	var wait sync.WaitGroup

	for key, gaps := range groups {
		key, gaps := key, gaps
		wait.Add(1)
		go func() {
			defer wait.Done()
			texts := make([]string, 0, len(gaps))
			for _, gap := range gaps {
				texts = append(texts, gap.source)
			}
			// 源语言按整组第一条的来源算。同一组里各字段的源语言理论上可能不同，
			// 但实际都来自同一个商品表单，混用的概率极低；真混了也只是让
			// 翻译服务自己去识别源语言，不会译错。
			translated, err := translator.TranslateBatch(texts, key.locale, gaps[0].sourceLocale, key.kind == localeFieldRichText)
			mu.Lock()
			defer mu.Unlock()
			if err != nil || len(translated) != len(gaps) {
				results = append(results, groupResult{gaps: gaps, failed: true})
				return
			}
			results = append(results, groupResult{gaps: gaps, texts: translated})
		}()
	}
	wait.Wait()

	filled := 0
	for _, result := range results {
		if result.failed {
			continue
		}
		for index, gap := range result.gaps {
			text := strings.TrimSpace(result.texts[index])
			// 批量接口对空译文是不报错的（那是刻意的，为了不让一格空拖垮整批），
			// 所以这里必须自己判：空译文写回去等于什么都没做，还会让
			// "已填"判定误以为这一格有内容，下次保存就再也不会补它了。
			if text == "" {
				continue
			}
			gap.field[gap.locale] = text
			filled++
		}
	}
	return filled
}

// fillProductLocales 收集商品各多语言字段的空缺并补译。
//
// previous 是更新时的旧商品（创建时传 nil）。旧值优先于机器翻译：
// 同一段文字不该每次保存都重译一遍——既费配额，译文还会漂移；
// 更要紧的是挡住"只提交部分语言的调用方把上次译好的内容洗掉"。
//
// 全程不返回错误。翻译失败、没配密钥、超时，商品都照常保存，
// 只是少了几门语言的译文——绝不能让第三方 API 挡住运营上架商品。
func (s *WriteService) fillProductLocales(input *CreateProductInput, previous *productdomain.Product) {
	if input == nil {
		return
	}

	// 先用旧值填空。这一步不依赖翻译服务，即便 DeepL 没配也该做。
	if previous != nil {
		seedFromPrevious(input.TitleJSON, previous.TitleJSON, localeFieldPlain)
		seedFromPrevious(input.DescriptionJSON, previous.DescriptionJSON, localeFieldPlain)
		seedFromPrevious(input.ContentJSON, previous.ContentJSON, localeFieldRichText)
		seedFromPrevious(input.InstructionsJSON, previous.InstructionsJSON, localeFieldRichText)
		seedSeoMetaFromPrevious(input.SeoMetaJSON, previous.SeoMetaJSON)
	}

	if s.translator == nil {
		return
	}

	fields := []translatableField{
		{values: input.TitleJSON, kind: localeFieldPlain},
		{values: input.DescriptionJSON, kind: localeFieldPlain},
		{values: input.ContentJSON, kind: localeFieldRichText},
		{values: input.InstructionsJSON, kind: localeFieldRichText},
	}
	// seo_meta 是两层结构 {keywords: {locale: str}, description: {locale: str}}，
	// 和其它字段形状不同，不能套扁平逻辑。注意它内层的 description 键
	// 和商品顶层的 description 撞名，别串了。
	fields = append(fields, seoMetaFields(input.SeoMetaJSON)...)
	// SKU 规格名买家在选规格时能看到，也要补。
	// 它的空缺形态是"键不存在"而不是空串——前端提交前会丢弃空值。
	for index := range input.SKUs {
		fields = append(fields, translatableField{
			values: input.SKUs[index].SpecValuesJSON,
			kind:   localeFieldPlain,
		})
	}

	fillMissingLocales(s.translator, fields)
}

// seedFromPrevious 用旧商品的同语言值填补请求里的空缺。
func seedFromPrevious(target map[string]interface{}, previous map[string]interface{}, kind localeFieldKind) {
	if target == nil || previous == nil {
		return
	}
	for _, locale := range constants.SupportedLocales {
		if !isBlankLocaleValue(target, locale, kind) {
			continue
		}
		if old := localeValue(previous, locale, kind); old != "" {
			target[locale] = old
		}
	}
}

// seoMetaFields 把 seo_meta 的两个内层字典摘出来。
func seoMetaFields(seoMeta map[string]interface{}) []translatableField {
	fields := make([]translatableField, 0, 2)
	for _, key := range []string{"keywords", "description"} {
		if inner, ok := seoMeta[key].(map[string]interface{}); ok && inner != nil {
			fields = append(fields, translatableField{values: inner, kind: localeFieldPlain})
		}
	}
	return fields
}

// seedSeoMetaFromPrevious 对 seo_meta 的两个内层字典分别做旧值填补。
func seedSeoMetaFromPrevious(target map[string]interface{}, previous map[string]interface{}) {
	if target == nil || previous == nil {
		return
	}
	for _, key := range []string{"keywords", "description"} {
		targetInner, targetOK := target[key].(map[string]interface{})
		previousInner, previousOK := previous[key].(map[string]interface{})
		if targetOK && previousOK {
			seedFromPrevious(targetInner, previousInner, localeFieldPlain)
		}
	}
}
