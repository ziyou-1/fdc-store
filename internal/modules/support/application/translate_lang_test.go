package application

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

// 站点支持的每一种语言都必须能映射到 DeepL 的语言码。
// 新增界面语言时这里会先红，提醒把映射补上——漏了会在运行时 400。
func TestDeepLTargetLangCoversEverySiteLocale(t *testing.T) {
	for _, locale := range constants.SupportedLocales {
		code, err := DeepLTargetLang(locale)
		if err != nil {
			t.Errorf("语言 %s 没有 DeepL 目标语言码: %v", locale, err)
			continue
		}
		if code == "" {
			t.Errorf("语言 %s 的目标语言码是空串", locale)
		}
		if DeepLSourceLang(locale) == "" {
			t.Errorf("语言 %s 没有 DeepL 源语言码", locale)
		}
	}
}

// 中文必须区分简繁，且用的是 DeepL 的写法而不是站点 locale。
// 直接把 "zh-CN" 当语言码发过去，DeepL 会 400。
func TestDeepLTargetLangUsesDeepLCodesNotSiteLocales(t *testing.T) {
	cases := map[string]string{
		constants.LocaleZhCN: "ZH-HANS",
		constants.LocaleZhTW: "ZH-HANT",
		constants.LocaleEnUS: "EN-US",
		constants.LocaleIdID: "ID",
	}
	for locale, want := range cases {
		got, err := DeepLTargetLang(locale)
		if err != nil {
			t.Fatalf("%s: %v", locale, err)
		}
		if got != want {
			t.Errorf("%s 应映射为 %s，实得 %s", locale, want, got)
		}
	}

	if _, err := DeepLTargetLang("fr-FR"); err == nil {
		t.Fatal("不支持的语言应当报错，而不是静默发一个错误的语言码")
	}
}

// 源语言码和目标语言码是两套表：DeepL 的 source_lang 不接受 EN-US / ZH-HANS。
func TestDeepLSourceLangUsesBaseCodes(t *testing.T) {
	cases := map[string]string{
		constants.LocaleZhCN: "ZH",
		constants.LocaleZhTW: "ZH",
		constants.LocaleEnUS: "EN",
		constants.LocaleIdID: "ID",
	}
	for locale, want := range cases {
		if got := DeepLSourceLang(locale); got != want {
			t.Errorf("%s 的源语言码应为 %s，实得 %s", locale, want, got)
		}
	}
	if got := DeepLSourceLang("fr-FR"); got != "" {
		t.Errorf("未知语言的源语言码应为空串（交给 DeepL 自动识别），实得 %s", got)
	}
}

// TranslateTo 必须把目标语言真的发出去，且 source_lang 为空时不带该字段。
func TestTranslateToSendsRequestedLanguages(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"translations":[{"text":"hasil"}]}`))
	}))
	defer server.Close()

	translator := &DeepLTranslator{authKey: "test-key", endpoint: server.URL, client: server.Client()}

	got, err := translator.TranslateTo("你好", "ID", "ZH")
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if got != "hasil" {
		t.Fatalf("译文 = %q", got)
	}
	if captured["target_lang"] != "ID" {
		t.Errorf("target_lang = %v，应为 ID", captured["target_lang"])
	}
	if captured["source_lang"] != "ZH" {
		t.Errorf("source_lang = %v，应为 ZH", captured["source_lang"])
	}

	// 源语言留空时不该带 source_lang 字段，否则 DeepL 会因为空值报错
	captured = nil
	if _, err := translator.TranslateTo("你好", "EN-US", "  "); err != nil {
		t.Fatalf("translate: %v", err)
	}
	if _, exists := captured["source_lang"]; exists {
		t.Error("源语言为空时不应发送 source_lang 字段")
	}
}

// 目标语言为空必须直接报错，不能发一个没有 target_lang 的请求出去。
func TestTranslateToRejectsEmptyTargetLang(t *testing.T) {
	translator := &DeepLTranslator{authKey: "k", endpoint: "http://127.0.0.1:1", client: http.DefaultClient}
	if _, err := translator.TranslateTo("hi", "   ", ""); err == nil {
		t.Fatal("空目标语言应当报错")
	}
}
