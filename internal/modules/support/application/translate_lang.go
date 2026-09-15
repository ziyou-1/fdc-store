package application

import (
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
)

// DeepL 的语言码和站点 locale 不同名，必须显式映射，不能拿 locale 直接当参数发过去。
//
// 尤其是中文：DeepL 用 ZH-HANS / ZH-HANT 区分简繁，而不是 ZH-CN / ZH-TW；
// 印尼语在 DeepL 侧就是 ID，没有地区后缀。发错语言码 DeepL 会直接 400。
var deeplTargetLangByLocale = map[string]string{
	constants.LocaleZhCN: "ZH-HANS",
	constants.LocaleZhTW: "ZH-HANT",
	constants.LocaleEnUS: "EN-US",
	constants.LocaleIdID: "ID",
}

// 源语言码。DeepL 的 source_lang 不接受 EN-US / ZH-HANS 这类带变体的值，
// 只收 EN / ZH 这样的基础码——目标语言和源语言用的是两套码表，别混用。
var deeplSourceLangByLocale = map[string]string{
	constants.LocaleZhCN: "ZH",
	constants.LocaleZhTW: "ZH",
	constants.LocaleEnUS: "EN",
	constants.LocaleIdID: "ID",
}

// DeepLTargetLang 把站点 locale 转成 DeepL 的目标语言码。
func DeepLTargetLang(locale string) (string, error) {
	code, ok := deeplTargetLangByLocale[strings.TrimSpace(locale)]
	if !ok {
		return "", fmt.Errorf("deepl: unsupported target locale %q", locale)
	}
	return code, nil
}

// DeepLSourceLang 把站点 locale 转成 DeepL 的源语言码。
// 返回空串表示不指定源语言，交给 DeepL 自动识别。
func DeepLSourceLang(locale string) string {
	return deeplSourceLangByLocale[strings.TrimSpace(locale)]
}
