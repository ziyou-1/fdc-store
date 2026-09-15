package productwrite

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

// fakeTranslator 记录收到的请求，并按 "<locale>:<原文>" 造译文。
type fakeTranslator struct {
	mu       sync.Mutex
	calls    int
	requests []fakeRequest
	err      error
	// blank 里的原文会被译成空串，用来验证"空译文不写回"。
	blank map[string]bool
}

type fakeRequest struct {
	texts        []string
	targetLocale string
	sourceLocale string
	richText     bool
}

func (f *fakeTranslator) TranslateBatch(texts []string, targetLocale, sourceLocale string, richText bool) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.requests = append(f.requests, fakeRequest{
		texts: append([]string(nil), texts...), targetLocale: targetLocale,
		sourceLocale: sourceLocale, richText: richText,
	})
	if f.err != nil {
		return nil, f.err
	}
	out := make([]string, 0, len(texts))
	for _, text := range texts {
		if f.blank[text] {
			out = append(out, "")
			continue
		}
		out = append(out, targetLocale+":"+text)
	}
	return out, nil
}

func (f *fakeTranslator) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// 只填了中文时，其余三门语言都应被补上，中文本身不动。
func TestFillMissingLocalesFillsOnlyGaps(t *testing.T) {
	title := map[string]interface{}{
		constants.LocaleZhCN: "会员月卡",
		constants.LocaleZhTW: "",
		constants.LocaleEnUS: "",
		constants.LocaleIdID: "",
	}
	translator := &fakeTranslator{}
	filled := fillMissingLocales(translator, []translatableField{{values: title, kind: localeFieldPlain}})

	if filled != 3 {
		t.Fatalf("应补 3 门语言，实得 %d", filled)
	}
	if title[constants.LocaleZhCN] != "会员月卡" {
		t.Fatalf("源语言被改动了: %v", title[constants.LocaleZhCN])
	}
	for _, locale := range []string{constants.LocaleZhTW, constants.LocaleEnUS, constants.LocaleIdID} {
		if got := title[locale]; got != locale+":会员月卡" {
			t.Errorf("%s 未被正确补译，实得 %v", locale, got)
		}
	}
}

// 已经有值的语言绝不能被覆盖——运营人工校对过的译文不能被下次保存冲掉。
func TestFillMissingLocalesNeverOverwritesExisting(t *testing.T) {
	title := map[string]interface{}{
		constants.LocaleZhCN: "会员月卡",
		constants.LocaleEnUS: "Monthly Pass",
		constants.LocaleIdID: "",
	}
	translator := &fakeTranslator{}
	fillMissingLocales(translator, []translatableField{{values: title, kind: localeFieldPlain}})

	if title[constants.LocaleEnUS] != "Monthly Pass" {
		t.Fatalf("已填的英文被覆盖成了 %v", title[constants.LocaleEnUS])
	}
	for _, request := range translator.requests {
		if request.targetLocale == constants.LocaleEnUS {
			t.Fatal("已填的语言不该被送去翻译，白花配额")
		}
	}
}

// 富文本的空壳（tiptap 里打字又删光）必须被当成空，否则永远不会被补译。
func TestFillMissingLocalesTreatsEmptyRichTextAsBlank(t *testing.T) {
	content := map[string]interface{}{
		constants.LocaleZhCN: "<p>正文</p>",
		constants.LocaleZhTW: "<p></p>",
		constants.LocaleEnUS: "<p><br></p>",
		constants.LocaleIdID: "   ",
	}
	translator := &fakeTranslator{}
	filled := fillMissingLocales(translator, []translatableField{{values: content, kind: localeFieldRichText}})

	if filled != 3 {
		t.Fatalf("三种空壳都应被补译，实得 %d", filled)
	}
	if len(translator.requests) == 0 || !translator.requests[0].richText {
		t.Fatal("富文本字段必须以富文本模式送出，否则 DeepL 会把标签里的 URL 当正文翻掉")
	}
}

// 富文本与纯文本不能混在同一次请求里：tag_handling 是请求级参数。
func TestFillMissingLocalesSeparatesRichTextFromPlain(t *testing.T) {
	title := map[string]interface{}{constants.LocaleZhCN: "标题", constants.LocaleIdID: ""}
	content := map[string]interface{}{constants.LocaleZhCN: "<p>正文</p>", constants.LocaleIdID: ""}
	translator := &fakeTranslator{}
	fillMissingLocales(translator, []translatableField{
		{values: title, kind: localeFieldPlain},
		{values: content, kind: localeFieldRichText},
	})

	// 3 门目标语言（zh-TW / en-US / id-ID）× 2 种文本类型 = 6 次。
	// 富文本和纯文本不能合并，因为 tag_handling 是整个请求级的参数。
	if translator.callCount() != 6 {
		t.Fatalf("3 门目标语言 × 2 种文本类型应发 6 次请求，实得 %d 次", translator.callCount())
	}
	for _, request := range translator.requests {
		if len(request.texts) != 1 {
			t.Errorf("每次请求应只带该类型的 1 条文本，实得 %d 条", len(request.texts))
		}
	}
	var sawPlain, sawRich bool
	for _, request := range translator.requests {
		if request.richText {
			sawRich = true
			if len(request.texts) != 1 || !strings.Contains(request.texts[0], "<p>") {
				t.Errorf("富文本请求内容不对: %v", request.texts)
			}
		} else {
			sawPlain = true
			if len(request.texts) != 1 || request.texts[0] != "标题" {
				t.Errorf("纯文本请求内容不对: %v", request.texts)
			}
		}
	}
	if !sawPlain || !sawRich {
		t.Fatal("两种模式的请求都应该出现")
	}
}

// 同一门语言的多个纯文本字段应打包成一次请求，而不是逐条翻。
func TestFillMissingLocalesBatchesFieldsPerLanguage(t *testing.T) {
	title := map[string]interface{}{constants.LocaleZhCN: "标题", constants.LocaleIdID: ""}
	description := map[string]interface{}{constants.LocaleZhCN: "描述", constants.LocaleIdID: ""}
	keywords := map[string]interface{}{constants.LocaleZhCN: "关键词", constants.LocaleIdID: ""}
	translator := &fakeTranslator{}
	fillMissingLocales(translator, []translatableField{
		{values: title, kind: localeFieldPlain},
		{values: description, kind: localeFieldPlain},
		{values: keywords, kind: localeFieldPlain},
	})

	// 关键在"每门语言一次"而不是"每字段一次"：3 字段 × 3 门语言若逐条翻是 9 次。
	if translator.callCount() != 3 {
		t.Fatalf("三个字段应按语言打包成 3 次请求（每门语言一次），实得 %d 次", translator.callCount())
	}
	for _, request := range translator.requests {
		if got := len(request.texts); got != 3 {
			t.Fatalf("每次请求应带 3 条文本，实得 %d", got)
		}
	}
}

// 翻译失败必须 fail-open：字段保持原样，不 panic、不阻断。
func TestFillMissingLocalesFailsOpen(t *testing.T) {
	title := map[string]interface{}{constants.LocaleZhCN: "标题", constants.LocaleIdID: ""}
	translator := &fakeTranslator{err: errors.New("deepl status 456: Quota exceeded")}

	filled := fillMissingLocales(translator, []translatableField{{values: title, kind: localeFieldPlain}})
	if filled != 0 {
		t.Fatalf("翻译失败时不该补上任何语言，实得 %d", filled)
	}
	if title[constants.LocaleIdID] != "" {
		t.Fatalf("翻译失败后字段被写脏: %v", title[constants.LocaleIdID])
	}
	if title[constants.LocaleZhCN] != "标题" {
		t.Fatal("翻译失败不该影响已有内容")
	}
}

// 没有翻译器（没配密钥或运营关掉了）时整段跳过，这是常态不是异常。
func TestFillMissingLocalesSkipsWhenTranslatorMissing(t *testing.T) {
	title := map[string]interface{}{constants.LocaleZhCN: "标题", constants.LocaleIdID: ""}
	if filled := fillMissingLocales(nil, []translatableField{{values: title, kind: localeFieldPlain}}); filled != 0 {
		t.Fatalf("没有翻译器时不该补任何语言，实得 %d", filled)
	}
	if title[constants.LocaleIdID] != "" {
		t.Fatal("没有翻译器时字段不该被改动")
	}
}

// 空译文不能写回：写回去等于什么都没做，还会让"已填"判定误判，
// 导致这一格以后再也不会被补译。
func TestFillMissingLocalesSkipsEmptyTranslation(t *testing.T) {
	title := map[string]interface{}{constants.LocaleZhCN: "标题", constants.LocaleIdID: ""}
	translator := &fakeTranslator{blank: map[string]bool{"标题": true}}

	filled := fillMissingLocales(translator, []translatableField{{values: title, kind: localeFieldPlain}})
	if filled != 0 {
		t.Fatalf("空译文不该计入已补，实得 %d", filled)
	}
	if title[constants.LocaleIdID] != "" {
		t.Fatalf("空译文被写回了: %v", title[constants.LocaleIdID])
	}
}

// 一门语言都没填时没有原文可译，直接跳过而不是拿空串去请求。
func TestFillMissingLocalesSkipsFieldWithNoSource(t *testing.T) {
	empty := map[string]interface{}{
		constants.LocaleZhCN: "", constants.LocaleZhTW: "",
		constants.LocaleEnUS: "", constants.LocaleIdID: "",
	}
	translator := &fakeTranslator{}
	if filled := fillMissingLocales(translator, []translatableField{{values: empty, kind: localeFieldPlain}}); filled != 0 {
		t.Fatalf("无原文可译时不该补任何语言，实得 %d", filled)
	}
	if translator.callCount() != 0 {
		t.Fatal("无原文可译时不该发出任何翻译请求")
	}
}

// 中文为空时用第一个非空语言当源——运营改用英文录入也要能工作。
func TestResolveSourceLocalePrefersChineseThenFallsBack(t *testing.T) {
	zh := map[string]interface{}{constants.LocaleZhCN: "中文", constants.LocaleEnUS: "English"}
	if got := resolveSourceLocale(zh, localeFieldPlain); got != constants.LocaleZhCN {
		t.Fatalf("有中文时应以中文为源，实得 %s", got)
	}

	en := map[string]interface{}{constants.LocaleZhCN: "  ", constants.LocaleEnUS: "English"}
	if got := resolveSourceLocale(en, localeFieldPlain); got != constants.LocaleEnUS {
		t.Fatalf("中文为空时应回落到第一个非空语言，实得 %s", got)
	}

	none := map[string]interface{}{constants.LocaleZhCN: ""}
	if got := resolveSourceLocale(none, localeFieldPlain); got != "" {
		t.Fatalf("全空时应返回空串，实得 %s", got)
	}
}

// 键不存在与值为空串都算空——spec_values 走前者，title 走后者。
func TestIsBlankLocaleValueCoversBothShapes(t *testing.T) {
	values := map[string]interface{}{
		constants.LocaleZhCN: "有值",
		constants.LocaleZhTW: "",
		constants.LocaleEnUS: "   ",
		// id-ID 键直接不存在
	}
	if isBlankLocaleValue(values, constants.LocaleZhCN, localeFieldPlain) {
		t.Error("有值的不该被判为空")
	}
	for _, locale := range []string{constants.LocaleZhTW, constants.LocaleEnUS, constants.LocaleIdID} {
		if !isBlankLocaleValue(values, locale, localeFieldPlain) {
			t.Errorf("%s 应被判为空", locale)
		}
	}
	// 非字符串值也算空，不能因为类型断言失败就当成有内容
	if !isBlankLocaleValue(map[string]interface{}{"zh-CN": 123}, constants.LocaleZhCN, localeFieldPlain) {
		t.Error("非字符串值应被判为空")
	}
}

// 旧值优先于机器翻译：更新时同一段文字不该每次保存都被重译。
func TestSeedFromPreviousFillsGapsBeforeTranslating(t *testing.T) {
	incoming := map[string]interface{}{
		constants.LocaleZhCN: "新标题",
		constants.LocaleEnUS: "",
		constants.LocaleIdID: "",
	}
	previous := map[string]interface{}{
		constants.LocaleEnUS: "Old English Title",
		constants.LocaleIdID: "",
	}
	seedFromPrevious(incoming, previous, localeFieldPlain)

	if incoming[constants.LocaleEnUS] != "Old English Title" {
		t.Fatalf("旧值未被用来填补空缺，实得 %v", incoming[constants.LocaleEnUS])
	}
	if incoming[constants.LocaleZhCN] != "新标题" {
		t.Fatal("请求里已有的值不该被旧值覆盖")
	}

	// 填补之后再补译，只有旧值也空的那门语言才会真的去翻
	translator := &fakeTranslator{}
	fillMissingLocales(translator, []translatableField{{values: incoming, kind: localeFieldPlain}})
	for _, request := range translator.requests {
		if request.targetLocale == constants.LocaleEnUS {
			t.Fatal("旧值已经填上的语言不该再送去翻译")
		}
	}
	if incoming[constants.LocaleIdID] != constants.LocaleIdID+":新标题" {
		t.Fatalf("旧值也空的语言应被补译，实得 %v", incoming[constants.LocaleIdID])
	}
}

// seo_meta 是两层结构，不能套扁平逻辑。
func TestSeoMetaFieldsExtractsInnerMaps(t *testing.T) {
	seoMeta := map[string]interface{}{
		"keywords":    map[string]interface{}{constants.LocaleZhCN: "关键词", constants.LocaleIdID: ""},
		"description": map[string]interface{}{constants.LocaleZhCN: "描述", constants.LocaleIdID: ""},
	}
	fields := seoMetaFields(seoMeta)
	if len(fields) != 2 {
		t.Fatalf("应摘出 keywords 与 description 两个内层字典，实得 %d", len(fields))
	}

	translator := &fakeTranslator{}
	filled := fillMissingLocales(translator, fields)
	if filled != 6 {
		t.Fatalf("两个内层字典各补 3 门语言，应为 6，实得 %d", filled)
	}

	// 形状不对时不该 panic，也不该被当成待翻字段
	if got := len(seoMetaFields(map[string]interface{}{"keywords": "不是字典"})); got != 0 {
		t.Fatalf("形状不对的 seo_meta 应被跳过，实得 %d", got)
	}
	if got := len(seoMetaFields(nil)); got != 0 {
		t.Fatalf("nil seo_meta 应被跳过，实得 %d", got)
	}
}
